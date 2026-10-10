package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RestoreWorkspaceV3(
	workspaceRoot string,
	artifactsRoot string,
	verified VerifiedCapsuleV3,
) error {
	if verified.Manifest.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("capsule: restore requires schema version 3")
	}

	if err := verified.Manifest.Validate(); err != nil {
		return fmt.Errorf("capsule: invalid v3 manifest: %w", err)
	}

	if len(verified.Manifest.GoModules) != 1 {
		return fmt.Errorf("capsule: v3 restore requires exactly one Go module")
	}

	if len(verified.Files) != len(verified.Manifest.Workspace.Files) {
		return fmt.Errorf("capsule: workspace file count mismatch")
	}

	var goSum []byte
	foundGoSum := false

	for i, file := range verified.Files {
		metadata := verified.Manifest.Workspace.Files[i]

		if file.Path != metadata.Path {
			return fmt.Errorf("capsule: workspace path mismatch")
		}

		if err := ValidateWorkspacePath(file.Path, file.Mode); err != nil {
			return err
		}

		if err := VerifyWorkspaceFileV2(metadata, file.Data); err != nil {
			return err
		}

		if file.Path == "go.sum" {
			goSum = file.Data
			foundGoSum = true
		}
	}

	if !foundGoSum {
		return fmt.Errorf("capsule: missing workspace/go.sum")
	}

	if err := VerifyGoModuleArtifactsV3(
		verified.Manifest.GoModules[0],
		verified.Artifacts,
		goSum,
	); err != nil {
		return fmt.Errorf("capsule: verify restored module: %w", err)
	}

	workspaceAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("capsule: resolve workspace root: %w", err)
	}

	artifactsAbs, err := filepath.Abs(artifactsRoot)
	if err != nil {
		return fmt.Errorf("capsule: resolve artifacts root: %w", err)
	}

	workspaceResolved, err := filepath.EvalSymlinks(workspaceAbs)
	if err != nil {
		return fmt.Errorf("capsule: resolve workspace symlinks: %w", err)
	}

	artifactsResolved, err := filepath.EvalSymlinks(artifactsAbs)
	if err != nil {
		return fmt.Errorf("capsule: resolve artifacts symlinks: %w", err)
	}

	workspaceAbs = workspaceResolved
	artifactsAbs = artifactsResolved

	relative, err := filepath.Rel(workspaceAbs, artifactsAbs)
	if err != nil {
		return fmt.Errorf("capsule: compare restore roots: %w", err)
	}

	reverse, err := filepath.Rel(artifactsAbs, workspaceAbs)
	if err != nil {
		return fmt.Errorf("capsule: compare restore roots: %w", err)
	}

	isSameOrDescendant := func(rel string) bool {
		if rel == "." {
			return true
		}

		return rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}

	if isSameOrDescendant(relative) || isSameOrDescendant(reverse) {
		return fmt.Errorf("capsule: restore roots overlap")
	}

	for _, root := range []string{workspaceRoot, artifactsRoot} {
		info, err := os.Lstat(root)
		if err != nil {
			return fmt.Errorf("capsule: inspect restore root: %w", err)
		}

		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("capsule: invalid restore root")
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}

		if len(entries) != 0 {
			return fmt.Errorf("capsule: restore root must be empty")
		}
	}

	if err := restoreWorkspaceFiles(
		workspaceRoot,
		verified.Manifest,
		verified.Files,
	); err != nil {
		return err
	}

	artifactRoot, err := os.OpenRoot(artifactsRoot)
	if err != nil {
		return fmt.Errorf("capsule: open artifacts root: %w", err)
	}
	defer artifactRoot.Close()

	for _, artifact := range verified.Manifest.GoModules[0].Artifacts {
		if len(artifact.Path) < len("module-0001.") {
			return fmt.Errorf("capsule: invalid artifact path")
		}

		ext := artifact.Path[len("module-0001"):]

		name := verified.Manifest.GoModules[0].Version + ext

		data := verified.Artifacts[artifact.Path]

		if SHA256(data) != artifact.SHA256 {
			return fmt.Errorf("capsule: artifact changed during restore")
		}

		f, err := artifactRoot.OpenFile(
			name,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0600,
		)

		if err != nil {
			return fmt.Errorf("capsule: create artifact: %w", err)
		}

		if _, err := f.Write(data); err != nil {
			f.Close()
			return fmt.Errorf("capsule: write artifact: %w", err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("capsule: close artifact: %w", err)
		}
	}

	return nil
}
