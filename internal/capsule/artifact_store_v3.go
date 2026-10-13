package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var artifactStoreNameV3 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ArtifactStoreV3 holds verified artifacts in a private temporary directory.
// It is not safe for concurrent use.
type ArtifactStoreV3 struct {
	dir    string
	root   *os.Root
	budget int64
	used   int64
	names  map[string]artifactRecordV3
	closed bool
}

type artifactRecordV3 struct {
	info   os.FileInfo
	size   int64
	digest string
}

// NewArtifactStoreV3 creates a private store with a strict byte budget.
func NewArtifactStoreV3(budget int64) (*ArtifactStoreV3, error) {
	if budget <= 0 {
		return nil, fmt.Errorf("capsule: invalid artifact store budget")
	}

	dir, err := os.MkdirTemp("", "rew-artifacts-v3-*")
	if err != nil {
		return nil, fmt.Errorf("capsule: create artifact store: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("capsule: open artifact store: %w", err)
	}

	return &ArtifactStoreV3{
		dir: dir, root: root, budget: budget,
		names: make(map[string]artifactRecordV3),
	}, nil
}

// Put writes exactly size bytes, verifies SHA-256 and publishes atomically.
// On failure, no artifact is published or charged against the budget.
func (s *ArtifactStoreV3) Put(
	name string, src io.Reader, size int64, expectedSHA256 string,
) (resultErr error) {
	if s == nil || s.closed {
		return fmt.Errorf("capsule: artifact store closed")
	}
	if !artifactStoreNameV3.MatchString(name) ||
		name == "." || name == ".." {
		return fmt.Errorf("capsule: invalid artifact name")
	}
	if _, exists := s.names[name]; exists {
		return fmt.Errorf("capsule: duplicate artifact")
	}
	if size < 0 || size > s.budget-s.used {
		return fmt.Errorf("capsule: artifact exceeds store budget")
	}
	if len(expectedSHA256) != 64 {
		return fmt.Errorf("capsule: invalid artifact digest")
	}
	digest, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("capsule: invalid artifact digest")
	}

	temp, err := os.CreateTemp(s.dir, ".partial-*")
	if err != nil {
		return fmt.Errorf("capsule: create artifact temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	hash := sha256.New()
	written, copyErr := io.CopyN(io.MultiWriter(temp, hash), src, size)
	closeErr := temp.Close()

	if copyErr != nil || written != size {
		return fmt.Errorf("capsule: incomplete artifact: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("capsule: close artifact: %w", closeErr)
	}
	if hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		return fmt.Errorf("capsule: artifact SHA-256 mismatch")
	}

	destination := filepath.Join(s.dir, name)
	if err := os.Link(tempName, destination); err != nil {
		return fmt.Errorf("capsule: publish verified artifact: %w", err)
	}

	// Roll back publication if a subsequent inspection fails.
	defer func() {
		if resultErr != nil {
			if cleanupErr := os.Remove(destination); cleanupErr != nil {
				resultErr = errors.Join(
					resultErr,
					fmt.Errorf("capsule: rollback published artifact: %w", cleanupErr),
				)
			}
		}
	}()

	published, err := s.root.Open(name)
	if err != nil {
		return fmt.Errorf("capsule: inspect published artifact: %w", err)
	}

	info, statErr := published.Stat()
	closeErr = published.Close()
	if statErr != nil || closeErr != nil {
		return fmt.Errorf("capsule: inspect published artifact: %v, %v", statErr, closeErr)
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return fmt.Errorf("capsule: invalid published artifact")
	}

	s.names[name] = artifactRecordV3{
		info:   info,
		size:   size,
		digest: expectedSHA256,
	}
	s.used += size
	return nil
}

// Open returns a reader for a previously verified, published artifact.
func (s *ArtifactStoreV3) Open(name string) (*os.File, error) {
	if s == nil || s.closed {
		return nil, fmt.Errorf("capsule: artifact store closed")
	}
	record, ok := s.names[name]
	if !ok {
		return nil, fmt.Errorf("capsule: artifact not verified")
	}

	f, err := s.root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("capsule: open verified artifact: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("capsule: stat verified artifact: %w", err)
	}

	if !info.Mode().IsRegular() ||
		info.Size() != record.size ||
		!os.SameFile(info, record.info) {
		f.Close()
		return nil, fmt.Errorf("capsule: artifact identity mismatch")
	}

	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, record.size+1))
	if err != nil ||
		n != record.size ||
		hex.EncodeToString(hash.Sum(nil)) != record.digest {
		f.Close()
		return nil, fmt.Errorf("capsule: artifact integrity mismatch")
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, fmt.Errorf("capsule: rewind verified artifact: %w", err)
	}

	return f, nil
}

func (s *ArtifactStoreV3) Used() int64 {
	if s == nil {
		return 0
	}
	return s.used
}

// Close invalidates the store and removes its temporary directory.
// Callers must close any readers previously returned by Open.
func (s *ArtifactStoreV3) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	rootErr := s.root.Close()
	removeErr := os.RemoveAll(s.dir)
	if rootErr != nil {
		return rootErr
	}
	return removeErr
}
