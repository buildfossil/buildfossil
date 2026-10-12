package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func requireReaderDockerE2EV3(t *testing.T) (
	context.Context,
	*client.Client,
) {
	t.Helper()

	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 90*time.Second,
	)
	t.Cleanup(cancel)

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dockerClient.Close(); err != nil {
			t.Errorf("close Docker client: %v", err)
		}
	})

	ensureSnapshotTestImage(t, dockerClient)
	return ctx, dockerClient
}

func readerVolumeExistsV3(
	ctx context.Context,
	dockerClient *client.Client,
	name string,
) (bool, error) {
	listed, err := dockerClient.VolumeList(
		ctx, client.VolumeListOptions{},
	)
	if err != nil {
		return false, err
	}

	for _, volume := range listed.Items {
		if volume.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func TestSnapshotReadersDockerUploadE2E(t *testing.T) {
	ctx, dockerClient := requireReaderDockerE2EV3(t)

	workspace := snapshotTestTAR(
		t, "main.go", "streamed-workspace\n",
	)
	modules := snapshotTestTAR(
		t, "module.txt", "streamed-module\n",
	)

	// Two independent reader implementations, neither passed as []byte.
	workspaceReader := bytes.NewReader(workspace)
	moduleReader := io.MultiReader(
		bytes.NewReader(modules[:len(modules)/2]),
		bytes.NewReader(modules[len(modules)/2:]),
	)

	volumes, err := CreateSnapshotVolumesFromReaders(
		ctx, dockerClient, goReplayImage,
		workspaceReader, moduleReader,
	)
	if err != nil {
		t.Fatalf("streamed Docker upload: %v", err)
	}

	t.Cleanup(func() {
		if err := volumes.Close(); err != nil {
			t.Errorf("cleanup streamed volumes: %v", err)
		}
	})

	if volumes.Workspace == "" || volumes.ModuleCache == "" ||
		volumes.Workspace == volumes.ModuleCache {
		t.Fatal("expected two distinct Docker volumes")
	}

	for _, name := range []string{
		volumes.Workspace, volumes.ModuleCache,
	} {
		exists, err := readerVolumeExistsV3(ctx, dockerClient, name)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("uploaded volume missing: %s", name)
		}
	}

	if err := volumes.Close(); err != nil {
		t.Fatalf("close streamed volumes: %v", err)
	}

	for _, name := range []string{
		volumes.Workspace, volumes.ModuleCache,
	} {
		exists, err := readerVolumeExistsV3(ctx, dockerClient, name)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("volume remains after Close: %s", name)
		}
	}
}

type failingSnapshotReaderV3 struct{}

func (failingSnapshotReaderV3) Read([]byte) (int, error) {
	return 0, errors.New("injected snapshot read failure")
}

func TestSnapshotReadersDockerRollbackE2E(t *testing.T) {
	ctx, dockerClient := requireReaderDockerE2EV3(t)

	workspaceName, err := newSnapshotVolumeName()
	if err != nil {
		t.Fatal(err)
	}
	moduleName, err := newSnapshotVolumeName()
	if err != nil {
		t.Fatal(err)
	}

	names := []string{workspaceName, moduleName}
	next := 0

	generateName := func() (string, error) {
		if next >= len(names) {
			return "", fmt.Errorf("unexpected extra volume")
		}
		name := names[next]
		next++
		return name, nil
	}

	workspace := snapshotTestTAR(
		t, "main.go", "valid-workspace\n",
	)

	volumes, uploadErr := createSnapshotVolumesFromReadersWithNameGenerator(
		ctx,
		dockerClient,
		goReplayImage,
		bytes.NewReader(workspace),
		failingSnapshotReaderV3{},
		generateName,
	)

	if volumes != nil {
		t.Cleanup(func() { _ = volumes.Close() })
		t.Fatal("returned volumes after failed upload")
	}
	if uploadErr == nil {
		t.Fatal("failed second reader unexpectedly accepted")
	}
	if next != 2 {
		t.Fatalf("created %d names, want 2", next)
	}

	for _, name := range names {
		exists, err := readerVolumeExistsV3(
			ctx, dockerClient, name,
		)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("volume leaked after reader error: %s", name)
		}
	}

	t.Logf("reader error triggered rollback: %v", uploadErr)
}
