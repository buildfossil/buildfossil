package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func TestSDKOfflineExecCancelAfterCommandStarted(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}
	if os.Geteuid() == 0 {
		t.Skip("offline runner rejects root")
	}

	fixture := makeOfflineGoFixture(t)

	dockerClient, err := NewSDKClient()
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		code int
		err  error
	}

	done := make(chan result, 1)

	go func() {
		code, err := RunWithSDKRuntimeOfflineExec(
			ctx,
			fixture.workspace,
			[]string{
				"/bin/sh",
				"-c",
				"echo BF_EXEC_STARTED; sleep 60",
			},
			fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
			io.Discard,
			io.Discard,
			&RuntimeSpec{
				Kind:    "go",
				Version: "1.26.6",
			},
			fixture.cacheRoot,
		)

		done <- result{code: code, err: err}
	}()

	var containerID string
	started := false

	deadline := time.Now().Add(45 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case result := <-done:
			t.Fatalf(
				"runner finished before cancellation: code=%d err=%v",
				result.code,
				result.err,
			)
		default:
		}

		listCtx, stopList := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)

		containers, listErr := dockerClient.ContainerList(
			listCtx,
			client.ContainerListOptions{All: true},
		)
		stopList()

		if listErr != nil {
			cancel()
			<-done
			t.Fatalf("list containers: %v", listErr)
		}

		for _, item := range containers.Items {
			if item.Labels["org.buildfossil.component"] != "replay" {
				continue
			}

			inspectCtx, stopInspect := context.WithTimeout(
				context.Background(),
				3*time.Second,
			)

			inspected, inspectErr := dockerClient.ContainerInspect(
				inspectCtx,
				item.ID,
				client.ContainerInspectOptions{},
			)
			stopInspect()

			if inspectErr != nil ||
				inspected.Container.HostConfig == nil {
				continue
			}

			matchesWorkspace := false

			for _, mount := range inspected.Container.HostConfig.Mounts {
				if mount.Source == fixture.workspace {
					matchesWorkspace = true
					break
				}
			}

			if !matchesWorkspace {
				continue
			}

			containerID = item.ID

			// Docker Top observes the processes in this container.
			// A running "sleep 60" proves the captured exec
			// has started, rather than merely the container PID 1.
			topCtx, stopTop := context.WithTimeout(
				context.Background(),
				3*time.Second,
			)

			top, topErr := dockerClient.ContainerTop(
				topCtx,
				containerID,
				client.ContainerTopOptions{},
			)
			stopTop()

			if topErr != nil {
				continue
			}

			for _, process := range top.Processes {
				for _, value := range process {
					if strings.Contains(value, "sleep 60") {
						started = true
						break
					}
				}
				if started {
					break
				}
			}
		}

		if started {
			break
		}

		time.Sleep(150 * time.Millisecond)
	}

	if !started {
		cancel()

		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Fatal("runner did not stop after test timeout")
		}

		t.Fatal("captured command was not observed running")
	}

	t.Log("CAPTURED COMMAND STARTED: PASS")

	cancel()

	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf(
				"expected context.Canceled, code=%d err=%v",
				result.code,
				result.err,
			)
		}

	case <-time.After(15 * time.Second):
		t.Fatal("runner did not return after cancellation")
	}

	t.Log("CANCELLATION: PASS")

	checkCtx, stopCheck := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer stopCheck()

	_, inspectErr := dockerClient.ContainerInspect(
		checkCtx,
		containerID,
		client.ContainerInspectOptions{},
	)

	if inspectErr == nil {
		t.Fatalf(
			"container still exists after cancellation: %s",
			containerID,
		)
	}

	if !errdefs.IsNotFound(inspectErr) {
		t.Fatalf(
			"unexpected container inspect error: %v",
			inspectErr,
		)
	}

	t.Log("CONTAINER CLEANUP: PASS")
}
