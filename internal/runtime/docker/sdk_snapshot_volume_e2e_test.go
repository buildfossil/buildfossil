package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func TestSDKSnapshotVolumePrototype(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cli, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cli.Close(); err != nil {
			t.Errorf("close Docker client: %v", err)
		}
	})

	image := "alpine:3.21"

	volumeName := fmt.Sprintf("buildfossil-snapshot-test-%d", time.Now().UnixNano())

	_, err = cli.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name: volumeName,
		Labels: map[string]string{
			"org.buildfossil.component": "test-snapshot",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, removeErr := cli.VolumeRemove(cleanupCtx, volumeName, client.VolumeRemoveOptions{})
		if removeErr != nil {
			t.Errorf("remove volume: %v", removeErr)
		}
	})

	options := newSecureContainerOptions(
		"",
		[]string{"/bin/sh", "-c", "sleep 60"},
		"10001:10001",
	)
	options.Platform = nil
	options.Config.Image = image
	options.Config.WorkingDir = "/"
	options.HostConfig.Mounts = []mount.Mount{{
		Type:   mount.TypeVolume,
		Source: volumeName,
		Target: "/snapshot",
	}}

	created, err := cli.ContainerCreate(ctx, options)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, removeErr := cli.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{Force: true},
		)
		if removeErr != nil {
			t.Errorf("remove container: %v", removeErr)
		}
	})

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)

	// Simulate a verified workspace file on the host.
	hostWorkspace := t.TempDir()
	hostFile := filepath.Join(hostWorkspace, "test.txt")

	original := []byte("verified-buildfossil-data\n")

	if err := os.WriteFile(hostFile, original, 0600); err != nil {
		t.Fatal(err)
	}

	// Snapshot the file into memory before uploading it to Docker.
	data, err := os.ReadFile(hostFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(data)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = cli.CopyToContainer(ctx, created.ID, client.CopyToContainerOptions{
		DestinationPath: "/snapshot",
		Content:         bytes.NewReader(archive.Bytes()),
	})
	if err != nil {
		t.Fatalf("copy snapshot to container: %v", err)
	}

	// Modify the original host file after uploading the snapshot.
	// The execution container must still see the original contents.
	if err := os.WriteFile(
		hostFile,
		[]byte("tampered-host-data\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	hostData, err := os.ReadFile(hostFile)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(hostData, data) {
		t.Fatal("host mutation did not change the source file")
	}

	readOnlyOptions := newSecureContainerOptions(
		"",
		[]string{"/bin/sh", "-c", "cat /snapshot/test.txt; if echo tamper > /snapshot/test.txt 2>/dev/null; then exit 42; fi"},
		"10001:10001",
	)
	readOnlyOptions.Platform = nil
	readOnlyOptions.Config.Image = image
	readOnlyOptions.Config.WorkingDir = "/"
	readOnlyOptions.HostConfig.Mounts = []mount.Mount{{
		Type:     mount.TypeVolume,
		Source:   volumeName,
		Target:   "/snapshot",
		ReadOnly: true,
	}}

	execContainer, err := cli.ContainerCreate(ctx, readOnlyOptions)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, removeErr := cli.ContainerRemove(
			cleanupCtx,
			execContainer.ID,
			client.ContainerRemoveOptions{Force: true},
		)
		if removeErr != nil {
			t.Errorf("remove execution container: %v", removeErr)
		}
	})

	if _, err := cli.ContainerStart(
		ctx,
		execContainer.ID,
		client.ContainerStartOptions{},
	); err != nil {
		t.Fatal(err)
	}

	waitResult := cli.ContainerWait(
		ctx, execContainer.ID, client.ContainerWaitOptions{},
	)

	select {
	case result := <-waitResult.Result:
		if result.StatusCode != 0 {
			t.Fatalf("execution exit code = %d", result.StatusCode)
		}
	case err := <-waitResult.Error:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	result, err := cli.CopyFromContainer(ctx, execContainer.ID, client.CopyFromContainerOptions{
		SourcePath: "/snapshot/test.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.Content.Close()

	raw, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(bytes.NewReader(raw))
	if _, err := tr.Next(); err != nil {
		t.Fatal(err)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, original) {
		t.Fatalf("snapshot changed: got %q, want %q", content, original)
	}

	t.Log("SNAPSHOT VOLUME PROTOTYPE: PASS")
}
