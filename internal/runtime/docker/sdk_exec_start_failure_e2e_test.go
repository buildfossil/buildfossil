package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestExecuteInContainerStartFailureE2E(t *testing.T) {
	ctx, dockerClient := requireReaderDockerE2EV3(t)

	createContainer := func(workspaceExists bool) string {
		t.Helper()

		script := "while :; do sleep 60; done"

		options := newSecureContainerOptions(
			"",
			[]string{"/bin/sh", "-c", script},
			"10001:10001",
		)
		options.Config.Image = goReplayImage
		options.Config.WorkingDir = "/"
		options.HostConfig.Mounts = nil

		if workspaceExists {
			if options.HostConfig.Tmpfs == nil {
				options.HostConfig.Tmpfs = make(map[string]string)
			}
			options.HostConfig.Tmpfs["/workspace"] = "rw,noexec,nosuid,nodev,size=1m,mode=1777"
		}

		created, err := dockerClient.ContainerCreate(ctx, options)
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(
				context.Background(), 15*time.Second,
			)
			defer cancel()

			if _, err := dockerClient.ContainerRemove(
				cleanupCtx,
				created.ID,
				client.ContainerRemoveOptions{Force: true},
			); err != nil {
				t.Errorf("remove test container: %v", err)
			}
		})

		if _, err := dockerClient.ContainerStart(
			ctx, created.ID, client.ContainerStartOptions{},
		); err != nil {
			t.Fatal(err)
		}

		return created.ID
	}

	t.Run("OCI startup failure", func(t *testing.T) {
		containerID := createContainer(false)

		_, _, _, err := executeInContainer(
			ctx,
			dockerClient,
			containerID,
			"10001:10001",
			[]string{"/bin/true"},
		)

		if err == nil {
			t.Fatal("expected infrastructure error when /workspace is missing")
		}

		if !strings.Contains(err.Error(), "did not report a started process") {
			t.Fatalf("unexpected infrastructure error: %v", err)
		}
	})

	t.Run("legitimate exit 127", func(t *testing.T) {
		containerID := createContainer(true)

		code, _, stderr, err := executeInContainer(
			ctx,
			dockerClient,
			containerID,
			"10001:10001",
			[]string{"/bin/sh", "-c", "exit 127"},
		)

		if err != nil {
			t.Fatalf("legitimate exit 127: %v", err)
		}
		if code != 127 {
			t.Fatalf("exit code = %d, want 127", code)
		}
		if len(stderr) != 0 {
			t.Fatalf("unexpected stderr: %q", stderr)
		}
	})

	t.Run("ordinary build failure", func(t *testing.T) {
		containerID := createContainer(true)

		code, _, stderr, err := executeInContainer(
			ctx,
			dockerClient,
			containerID,
			"10001:10001",
			[]string{
				"/bin/sh",
				"-c",
				"printf 'build failed\\n' >&2; exit 1",
			},
		)

		if err != nil {
			t.Fatalf("ordinary command failure: %v", err)
		}
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if string(stderr) != "build failed\n" {
			t.Fatalf("stderr = %q, want build failure", stderr)
		}
	})
}
