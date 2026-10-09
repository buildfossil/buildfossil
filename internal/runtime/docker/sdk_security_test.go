package docker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestSDKContainerSecurityConfiguration(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	workspace := t.TempDir()

	created, err := dockerClient.ContainerCreate(
		ctx,
		newSecureContainerOptions(
			workspace,
			[]string{"/bin/sh", "-c", "sleep 5"},
			"65534:65534",
		),
	)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cleanupCancel()

		_, err := dockerClient.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{Force: true},
		)
		if err != nil {
			t.Errorf("remove container: %v", err)
		}
	}()

	inspected, err := dockerClient.ContainerInspect(
		ctx,
		created.ID,
		client.ContainerInspectOptions{},
	)
	if err != nil {
		t.Fatalf("inspect container: %v", err)
	}

	host := inspected.Container.HostConfig
	if host == nil {
		t.Fatal("missing HostConfig")
	}

	if host.Privileged {
		t.Error("container must not be privileged")
	}

	if string(host.UsernsMode) == "host" {
		t.Error("container must not disable user namespace remapping")
	}

	if len(host.Devices) != 0 {
		t.Errorf("unexpected host devices: %+v", host.Devices)
	}

	if len(host.Binds) != 0 {
		t.Errorf("unexpected additional bind mounts: %+v", host.Binds)
	}

	if host.PidMode.IsHost() {
		t.Error("container must not share host PID namespace")
	}

	if host.IpcMode.IsHost() {
		t.Error("container must not share host IPC namespace")
	}

	if host.LogConfig.Type != "json-file" {
		t.Errorf(
			"log driver = %q; want json-file",
			host.LogConfig.Type,
		)
	}

	if host.LogConfig.Type != "json-file" {
		t.Errorf(
			"log driver = %q; want json-file",
			host.LogConfig.Type,
		)
	}

	if got := host.LogConfig.Config["max-size"]; got != "1m" {
		t.Errorf("log max-size = %q; want 1m", got)
	}

	if got := host.LogConfig.Config["max-file"]; got != "1" {
		t.Errorf("log max-file = %q; want 1", got)
	}

	if string(host.NetworkMode) != "none" {
		t.Errorf("network mode = %q; want none", host.NetworkMode)
	}

	if !host.ReadonlyRootfs {
		t.Error("root filesystem is not read-only")
	}

	if len(host.CapDrop) != 1 || host.CapDrop[0] != "ALL" {
		t.Errorf("unexpected capabilities: %v", host.CapDrop)
	}

	if len(host.SecurityOpt) != 1 ||
		host.SecurityOpt[0] != "no-new-privileges" {
		t.Errorf("unexpected security options: %v", host.SecurityOpt)
	}

	if host.PidsLimit == nil || *host.PidsLimit != 64 {
		t.Errorf("unexpected PIDs limit: %v", host.PidsLimit)
	}

	if host.Memory != 256*1024*1024 {
		t.Errorf("unexpected memory limit: %d", host.Memory)
	}

	if host.NanoCPUs != 1_000_000_000 {
		t.Errorf("unexpected CPU limit: %d", host.NanoCPUs)
	}

	if len(host.Mounts) != 1 {
		t.Fatalf("expected one mount; got %d", len(host.Mounts))
	}

	if !host.Mounts[0].ReadOnly ||
		host.Mounts[0].Target != "/workspace" {
		t.Errorf("workspace mount is not read-only: %+v", host.Mounts[0])
	}
}
