package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func snapshotTestTAR(t *testing.T, name, value string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	data := []byte(value)

	if err := writer.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0644,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}

func TestSnapshotVolumesEndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = dockerClient.Close()
	})

	workspaceTAR := snapshotTestTAR(
		t, "main.go", "original-workspace\n",
	)
	moduleTAR := snapshotTestTAR(
		t, "module.txt", "original-module\n",
	)

	volumes, err := CreateSnapshotVolumes(
		ctx,
		dockerClient,
		"alpine:3.21",
		workspaceTAR,
		moduleTAR,
	)
	if err != nil {
		t.Fatalf("create snapshot volumes: %v", err)
	}

	t.Cleanup(func() {
		if err := volumes.Close(); err != nil {
			t.Errorf("cleanup snapshot volumes: %v", err)
		}
	})

	if volumes.Workspace == "" ||
		volumes.ModuleCache == "" ||
		volumes.Workspace == volumes.ModuleCache {
		t.Fatal("expected two distinct Docker volumes")
	}

	// Mutate the input archives after uploading them.
	// Docker volumes must retain their uploaded content.
	for i := range workspaceTAR {
		workspaceTAR[i] = 0
	}
	for i := range moduleTAR {
		moduleTAR[i] = 0
	}

	options := newSecureContainerOptions(
		"",
		[]string{
			"/bin/sh",
			"-ec",
			"cat /workspace/main.go; cat /module-cache-source/module.txt; " +
				"if echo tamper > /workspace/main.go 2>/dev/null; then exit 42; fi; " +
				"if echo tamper > /module-cache-source/module.txt 2>/dev/null; then exit 43; fi",
		},
		"10001:10001",
	)

	options.Platform = nil
	options.Config.Image = "alpine:3.21"
	options.Config.WorkingDir = "/"

	options.HostConfig.Mounts = []mount.Mount{
		{
			Type:     mount.TypeVolume,
			Source:   volumes.Workspace,
			Target:   "/workspace",
			ReadOnly: true,
			VolumeOptions: &mount.VolumeOptions{
				NoCopy: true,
			},
		},
		{
			Type:     mount.TypeVolume,
			Source:   volumes.ModuleCache,
			Target:   "/module-cache-source",
			ReadOnly: true,
			VolumeOptions: &mount.VolumeOptions{
				NoCopy: true,
			},
		},
	}

	created, err := dockerClient.ContainerCreate(ctx, options)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if created.ID == "" {
			return
		}

		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
		defer cancel()

		_, err := dockerClient.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{Force: true},
		)

		if err != nil {
			t.Errorf("remove execution container: %v", err)
		}
	})

	if _, err := dockerClient.ContainerStart(
		ctx,
		created.ID,
		client.ContainerStartOptions{},
	); err != nil {
		t.Fatal(err)
	}

	wait := dockerClient.ContainerWait(
		ctx,
		created.ID,
		client.ContainerWaitOptions{},
	)

	select {
	case result := <-wait.Result:
		if result.StatusCode != 0 {
			t.Fatalf("execution exit code = %d", result.StatusCode)
		}
	case err := <-wait.Error:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	for _, item := range []struct {
		path string
		want string
	}{
		{"/workspace/main.go", "original-workspace\n"},
		{"/module-cache-source/module.txt", "original-module\n"},
	} {
		result, err := dockerClient.CopyFromContainer(
			ctx,
			created.ID,
			client.CopyFromContainerOptions{
				SourcePath: item.path,
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		raw, readErr := io.ReadAll(result.Content)
		closeErr := result.Content.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}

		reader := tar.NewReader(bytes.NewReader(raw))
		if _, err := reader.Next(); err != nil {
			t.Fatal(err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		if strings.TrimSpace(string(data)) !=
			strings.TrimSpace(item.want) {
			t.Fatalf(
				"%s = %q; want %q",
				item.path, data, item.want,
			)
		}
	}

	// Execution containers must be removed before their volumes.
	cleanupCtx, cleanupCancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)

	_, removeErr := dockerClient.ContainerRemove(
		cleanupCtx,
		created.ID,
		client.ContainerRemoveOptions{Force: true},
	)

	cleanupCancel()

	if removeErr != nil {
		t.Fatalf("remove execution container: %v", removeErr)
	}

	// Mark the container as removed so t.Cleanup won't remove it again.
	created.ID = ""

	// The first call removes both volumes.
	if err := volumes.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	// Repeated Close must be safe.
	if err := volumes.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	t.Log("TWO SNAPSHOT VOLUMES: PASS")
}

func TestSnapshotVolumesRollbackOnSecondUploadFailure(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	workspaceTAR := snapshotTestTAR(
		t, "main.go", "valid-workspace\n",
	)

	// Deliberately invalid TAR to force the second upload to fail.
	invalidModuleTAR := []byte("not a valid tar archive")

	volumes, err := CreateSnapshotVolumes(
		ctx,
		dockerClient,
		"alpine:3.21",
		workspaceTAR,
		invalidModuleTAR,
	)

	if err == nil {
		if volumes != nil {
			_ = volumes.Close()
		}
		t.Fatal("expected second snapshot upload failure")
	}

	if volumes != nil {
		t.Fatal("manager returned volumes after failure")
	}

	listed, err := dockerClient.VolumeList(
		ctx,
		client.VolumeListOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, volume := range listed.Items {
		if volume.Labels["org.buildfossil.component"] == "snapshot" {
			t.Fatalf(
				"snapshot volume leaked after rollback: %s",
				volume.Name,
			)
		}
	}
}

func TestSnapshotVolumesCloseAfterCancellation(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	volumes, err := CreateSnapshotVolumes(
		ctx,
		dockerClient,
		"alpine:3.21",
		snapshotTestTAR(t, "main.go", "workspace\n"),
		snapshotTestTAR(t, "module.txt", "module\n"),
	)
	if err != nil {
		t.Fatal(err)
	}

	workspaceName := volumes.Workspace
	moduleName := volumes.ModuleCache

	// Simulate replay cancellation after snapshot preparation.
	cancel()

	if err := volumes.Close(); err != nil {
		t.Fatalf("cleanup after cancellation: %v", err)
	}

	// Verify both volumes were actually removed.
	checkCtx, checkCancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer checkCancel()

	listed, err := dockerClient.VolumeList(
		checkCtx,
		client.VolumeListOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, volume := range listed.Items {
		if volume.Name == workspaceName ||
			volume.Name == moduleName {
			t.Fatalf(
				"volume survived cancellation cleanup: %s",
				volume.Name,
			)
		}
	}

	if err := volumes.Close(); err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}

	t.Log("CANCELLATION CLEANUP: PASS")
}
