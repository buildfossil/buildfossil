package docker

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestNewRunID(t *testing.T) {
	first, err := newRunID()
	if err != nil {
		t.Fatal(err)
	}

	second, err := newRunID()
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 32 {
		t.Fatalf("run ID length = %d; want 32", len(first))
	}

	decoded, err := hex.DecodeString(first)
	if err != nil || len(decoded) != 16 {
		t.Fatalf("invalid run ID: %q, error: %v", first, err)
	}

	if first == second {
		t.Fatal("two generated run IDs are identical")
	}
}

func TestSDKRunContainerHasRunID(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	workspace := t.TempDir()
	user := fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid())

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	finished := make(chan error, 1)

	go func() {
		_, err := RunWithSDK(
			ctx,
			workspace,
			[]string{"/bin/sh", "-c", "sleep 10"},
			user,
			io.Discard,
			io.Discard,
		)
		finished <- err
	}()

	var containerID string
	var runID string

	deadline := time.Now().Add(8 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case err := <-finished:
			t.Fatalf("container finished before inspection: %v", err)
		default:
		}

		listCtx, cancelList := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)

		containers, listErr := dockerClient.ContainerList(
			listCtx,
			client.ContainerListOptions{All: false},
		)
		cancelList()

		if listErr != nil {
			t.Fatalf("list containers: %v", listErr)
		}

		for _, item := range containers.Items {
			if item.Labels["org.buildfossil.component"] != "replay" {
				continue
			}

			// Идентифицируем контейнер по его workspace,
			// а не по общему label.
			inspectCtx, cancelInspect := context.WithTimeout(
				context.Background(),
				2*time.Second,
			)

			inspected, inspectErr := dockerClient.ContainerInspect(
				inspectCtx,
				item.ID,
				client.ContainerInspectOptions{},
			)
			cancelInspect()

			if inspectErr != nil {
				continue
			}

			if inspected.Container.HostConfig == nil {
				continue
			}

			for _, mount := range inspected.Container.HostConfig.Mounts {
				if mount.Source == workspace {
					containerID = item.ID
					runID = item.Labels["org.buildfossil.run-id"]
					break
				}
			}
		}

		if containerID != "" {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	if containerID == "" {
		cancel()
		<-finished
		t.Fatal("test container not found")
	}

	if len(runID) != 32 {
		cancel()
		<-finished
		t.Fatalf("invalid run ID: %q", runID)
	}

	if _, err := hex.DecodeString(runID); err != nil {
		cancel()
		<-finished
		t.Fatalf("run ID is not hexadecimal: %v", err)
	}

	if strings.Trim(runID, "0") == "" {
		cancel()
		<-finished
		t.Fatal("run ID contains only zeros")
	}

	cancel()

	select {
	case <-finished:
	case <-time.After(8 * time.Second):
		t.Fatal("SDK did not terminate after cancellation")
	}
}
