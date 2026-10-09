package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func TestSDKContainerCleanup(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	// Используем отдельную временную папку, чтобы найти
	// контейнер именно этого теста по bind mount.
	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code, err := RunWithSDK(
		ctx,
		workspace,
		[]string{"/bin/sh", "-c", "exit 23"},
		user,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("RunWithSDK: %v", err)
	}
	if code != 23 {
		t.Fatalf("exit code = %d; want 23", code)
	}

	containers, err := dockerClient.ContainerList(
		context.Background(),
		client.ContainerListOptions{All: true},
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range containers.Items {
		inspected, err := dockerClient.ContainerInspect(
			context.Background(),
			c.ID,
			client.ContainerInspectOptions{},
		)
		if err != nil {
			continue
		}

		if inspected.Container.HostConfig == nil {
			continue
		}

		for _, m := range inspected.Container.HostConfig.Mounts {
			if m.Source == workspace {
				t.Fatalf("SDK left container behind: %s", c.ID)
			}
		}
	}

	_ = container.WaitConditionNotRunning
}

func TestSDKContainerCleanupOnCancellation(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	ctx, cancel := context.WithTimeout(
		context.Background(),
		500*time.Millisecond,
	)
	defer cancel()

	start := time.Now()

	_, err := RunWithSDK(
		ctx,
		workspace,
		[]string{"/bin/sh", "-c", "sleep 30"},
		user,
		io.Discard,
		io.Discard,
	)

	if err == nil {
		t.Fatal("expected cancellation error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("cancellation took too long: %s", elapsed)
	}

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	checkCtx, checkCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer checkCancel()

	containers, err := dockerClient.ContainerList(
		checkCtx,
		client.ContainerListOptions{All: true},
	)
	if err != nil {
		t.Fatalf("list containers after cancellation: %v", err)
	}

	for _, c := range containers.Items {
		inspected, inspectErr := dockerClient.ContainerInspect(
			checkCtx,
			c.ID,
			client.ContainerInspectOptions{},
		)
		if inspectErr != nil {
			t.Fatalf("inspect container %s: %v", c.ID, inspectErr)
		}

		if inspected.Container.HostConfig == nil {
			continue
		}

		for _, m := range inspected.Container.HostConfig.Mounts {
			if m.Source == workspace {
				t.Fatalf(
					"container left behind after cancellation: %s",
					c.ID,
				)
			}
		}
	}
}
