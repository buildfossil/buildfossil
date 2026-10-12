package replay

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

// snapshotFilesV3 owns both prepared TAR readers and their private
// temporary directory. Not safe for concurrent use.
type snapshotFilesV3 struct {
	directory string
	workspace *os.File
	modules   *os.File
}

func (s *snapshotFilesV3) Close() error {
	if s == nil {
		return nil
	}

	var err error
	if s.workspace != nil {
		err = errors.Join(err, s.workspace.Close())
		s.workspace = nil
	}
	if s.modules != nil {
		err = errors.Join(err, s.modules.Close())
		s.modules = nil
	}
	if s.directory != "" {
		err = errors.Join(err, os.RemoveAll(s.directory))
		s.directory = ""
	}
	return err
}

// prepareSnapshotFilesV3 completes both TAR archives before returning
// any reader to the caller. On failure, all partial files are removed.
func prepareSnapshotFilesV3(
	verified capsule.VerifiedCapsuleV3,
) (_ *snapshotFilesV3, resultErr error) {
	directory, err := os.MkdirTemp("", "buildfossil-replay-v3-*")
	if err != nil {
		return nil, fmt.Errorf("replay: create snapshot directory: %w", err)
	}

	result := &snapshotFilesV3{directory: directory}

	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()

	writeArchive := func(
		name string,
		build func(io.Writer, capsule.VerifiedCapsuleV3) error,
	) (*os.File, error) {
		filename := filepath.Join(directory, name)

		file, err := os.OpenFile(
			filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600,
		)
		if err != nil {
			return nil, err
		}

		if err := build(file, verified); err != nil {
			return nil, errors.Join(err, file.Close())
		}

		info, err := file.Stat()
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}

		if !info.Mode().IsRegular() ||
			info.Size() == 0 ||
			info.Size() > capsule.MaxCapsuleSizeV3 {
			return nil, errors.Join(
				fmt.Errorf("replay: invalid snapshot TAR size: %s", name),
				file.Close(),
			)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, errors.Join(err, file.Close())
		}

		return file, nil
	}

	result.workspace, err = writeArchive(
		"workspace.tar", capsule.WriteWorkspaceSnapshotV3,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"replay: prepare workspace TAR: %w", err,
		)
	}

	result.modules, err = writeArchive(
		"modules.tar", capsule.WriteGoModuleSnapshotV3,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"replay: prepare module TAR: %w", err,
		)
	}

	return result, nil
}

// prepareSnapshotFilesFromStoreV3 prepares both TARs from a
// verified disk-backed capsule before returning their readers.
func prepareSnapshotFilesFromStoreV3(
	verified capsule.VerifiedStoredCapsuleV3,
) (_ *snapshotFilesV3, resultErr error) {
	directory, err := os.MkdirTemp("", "buildfossil-replay-v3-*")
	if err != nil {
		return nil, fmt.Errorf("replay: create snapshot directory: %w", err)
	}

	result := &snapshotFilesV3{directory: directory}

	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()

	writeArchive := func(
		name string,
		build func(io.Writer) error,
	) (*os.File, error) {
		filename := filepath.Join(directory, name)

		file, err := os.OpenFile(
			filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600,
		)
		if err != nil {
			return nil, err
		}

		if err := build(file); err != nil {
			return nil, errors.Join(err, file.Close())
		}

		info, err := file.Stat()
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}

		if !info.Mode().IsRegular() ||
			info.Size() == 0 ||
			info.Size() > capsule.MaxCapsuleSizeV3 {
			return nil, errors.Join(
				fmt.Errorf("replay: invalid snapshot TAR size: %s", name),
				file.Close(),
			)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, errors.Join(err, file.Close())
		}

		return file, nil
	}

	result.workspace, err = writeArchive(
		"workspace.tar", func(w io.Writer) error {
			return capsule.WriteWorkspaceSnapshotV3(
				w,
				capsule.VerifiedCapsuleV3{
					Manifest: verified.Manifest,
					Files:    verified.Files,
				},
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"replay: prepare workspace TAR: %w", err,
		)
	}

	result.modules, err = writeArchive(
		"modules.tar", func(w io.Writer) error {
			return capsule.WriteGoModuleSnapshotFromStoreV3(
				w, verified,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"replay: prepare module TAR: %w", err,
		)
	}

	return result, nil
}

// closeSnapshotFilesV3 preserves an execution error while reporting
// failures to close or remove snapshot files.
func closeSnapshotFilesV3(s *snapshotFilesV3, runErr error) error {
	if cleanupErr := s.Close(); cleanupErr != nil {
		return errors.Join(
			runErr,
			fmt.Errorf("replay: cleanup snapshot files: %w", cleanupErr),
		)
	}
	return runErr
}
