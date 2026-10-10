package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func snapshotGoFixtureTAR(
	t *testing.T,
	root string,
	names []string,
) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)

	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}

		header := &tar.Header{
			Name:     path.Clean(name),
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}

		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}

		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}

func makeSnapshotGoArchives(t *testing.T) ([]byte, []byte) {
	t.Helper()

	fixture := makeOfflineGoFixture(t)

	workspaceTAR := snapshotGoFixtureTAR(
		t,
		fixture.workspace,
		[]string{"go.mod", "go.sum", "main.go"},
	)

	moduleDirectory := path.Join(
		"cache",
		"download",
		"example.com",
		"buildfossil",
		"fixture",
		"@v",
	)

	moduleNames := make([]string, 0, 3)
	for _, ext := range []string{".zip", ".mod", ".info"} {
		moduleNames = append(
			moduleNames,
			path.Join(moduleDirectory, testGoModuleVersion+ext),
		)
	}

	moduleTAR := snapshotGoFixtureTAR(
		t,
		fixture.cacheRoot,
		moduleNames,
	)

	return workspaceTAR, moduleTAR
}

var _ = fmt.Sprintf

func TestSnapshotGoArchives(t *testing.T) {
	workspaceTAR, moduleTAR := makeSnapshotGoArchives(t)

	if len(workspaceTAR) == 0 || len(moduleTAR) == 0 {
		t.Fatal("empty Go snapshot archives")
	}

	t.Logf(
		"workspace TAR: %d bytes; module TAR: %d bytes",
		len(workspaceTAR),
		len(moduleTAR),
	)
}

func TestSnapshotOfflineGoBuild(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspaceTAR, moduleTAR := makeSnapshotGoArchives(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		180*time.Second,
	)
	defer cancel()

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = dockerClient.Close()
	})

	runtime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	image, err := selectReplayImage(runtime)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureReplayImage(ctx, dockerClient, image); err != nil {
		t.Fatalf("prepare Go image: %v", err)
	}

	volumes, err := CreateSnapshotVolumes(
		ctx,
		dockerClient,
		image,
		workspaceTAR,
		moduleTAR,
	)
	if err != nil {
		t.Fatalf("create snapshot volumes: %v", err)
	}

	t.Cleanup(func() {
		if err := volumes.Close(); err != nil {
			t.Errorf("cleanup volumes: %v", err)
		}
	})

	options := newSecureContainerOptions(
		"",
		[]string{"/bin/sh", "-c", "while :; do sleep 60; done"},
		"10001:10001",
	)

	options.Config.Image = image

	options.Config.Env = []string{
		"PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TMPDIR=/tmp",
		"GOCACHE=/gocache",
		"GOMODCACHE=/gomodcache",
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"CGO_ENABLED=0",
	}

	options.HostConfig.Tmpfs = map[string]string{
		"/tmp":        "rw,nosuid,nodev,size=64m,mode=1777",
		"/gocache":    "rw,nosuid,nodev,size=64m,mode=1777",
		"/gomodcache": "rw,nosuid,nodev,size=64m,mode=1777",
	}

	// Replace the default host bind mount with two Docker volumes.
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

	// Registered after volume cleanup: runs before it.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
		defer cleanupCancel()

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

	for _, step := range []struct {
		name string
		argv []string
	}{
		{
			name: "copy module cache",
			argv: []string{
				"/bin/sh",
				"-ec",
				"cp -R /module-cache-source/. /gomodcache/",
			},
		},
		{
			name: "offline module download",
			argv: []string{"go", "mod", "download"},
		},
		{
			name: "offline Go build",
			argv: []string{
				"go",
				"build",
				"-mod=readonly",
				"-o",
				"/tmp/offline-build",
				".",
			},
		},
	} {
		code, _, stderr, err := executeInContainer(
			ctx,
			dockerClient,
			created.ID,
			"10001:10001",
			step.argv,
		)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if code != 0 {
			t.Fatalf(
				"%s: exit=%d, stderr=%s",
				step.name,
				code,
				stderr,
			)
		}
		if strings.Contains(string(stderr), "module lookup disabled") {
			t.Fatalf("%s attempted network module lookup: %s", step.name, stderr)
		}

		t.Logf("%s: PASS", step.name)
	}

	// A captured build failure must remain a command result,
	// not be treated as a Docker infrastructure error.
	code, stdout, stderr, err := executeInContainer(
		ctx,
		dockerClient,
		created.ID,
		"10001:10001",
		[]string{
			"/bin/sh",
			"-c",
			"printf 'buildfossil-snapshot-error\\n' >&2; exit 23",
		},
	)
	if err != nil {
		t.Fatalf("snapshot failure execution: %v", err)
	}

	if code != 23 {
		t.Fatalf("failure exit code = %d; want 23", code)
	}

	if len(stdout) != 0 {
		t.Fatalf("unexpected failure stdout: %q", stdout)
	}

	if string(stderr) != "buildfossil-snapshot-error\n" {
		t.Fatalf("unexpected failure stderr: %q", stderr)
	}

	t.Log("snapshot stderr and exit code: PASS")

	t.Log("SNAPSHOT OFFLINE GO BUILD: PASS")
}
