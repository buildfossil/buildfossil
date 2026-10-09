package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func TestReplayCLISIGINT(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("SIGINT E2E requires linux/amd64")
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
		Platform: capsule.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
		Execution: capsule.Execution{
			Argv: []string{
				"/bin/sh",
				"-c",
				"sleep 60; echo SHOULD_NOT_FINISH >&2; exit 23",
			},
			WorkingDir: ".",
			ExitCode:   23,
			Stderr:     "SHOULD_NOT_FINISH\n",
		},
	}

	files := []capsule.WorkspaceFile{
		{
			Path: "go.mod",
			Data: []byte("module example.com/signal-test\n"),
			Mode: 0600,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(
		&archive,
		manifest,
		files,
	); err != nil {
		t.Fatalf("write capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "signal-test.bfc")

	if err := os.WriteFile(capsulePath, archive.Bytes(), 0600); err != nil {
		t.Fatalf("save capsule: %v", err)
	}

	t.Logf("SIGINT test capsule prepared: %s", capsulePath)

	// Собираем настоящий CLI, а не запускаем runReplay() внутри теста.
	binary := filepath.Join(t.TempDir(), "buildfossil")

	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	dockerClient, err := client.New(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Fatalf("Docker client: %v", err)
	}
	defer dockerClient.Close()

	runCtx, cancelRun := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancelRun()

	cmd := exec.CommandContext(
		runCtx,
		binary,
		"replay",
		"--v2",
		"--allow-command",
		capsulePath,
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		t.Fatalf("start CLI: %v", err)
	}

	finished := make(chan error, 1)
	go func() {
		finished <- cmd.Wait()
	}()

	// Находим контейнер этого теста по уникальной команде.
	const marker = "SHOULD_NOT_FINISH"

	var containerID string
	deadline := time.Now().Add(12 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case err := <-finished:
			t.Fatalf("CLI exited before SIGINT: %v\n%s", err, output.String())
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
			t.Fatalf("list Docker containers: %v", listErr)
		}

		for _, item := range containers.Items {
			if item.Labels["org.buildfossil.component"] != "replay" {
				continue
			}

			if strings.Contains(item.Command, marker) {
				containerID = item.ID
				break
			}
		}

		if containerID != "" {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	if containerID == "" {
		_ = cmd.Process.Kill()
		<-finished
		t.Fatalf("Replay container did not start\n%s", output.String())
	}

	runningCtx, cancelRunning := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelRunning()

	inspected, err := dockerClient.ContainerInspect(
		runningCtx,
		containerID,
		client.ContainerInspectOptions{},
	)
	if err != nil {
		t.Fatalf("inspect running container: %v", err)
	}

	if inspected.Container.State == nil ||
		!inspected.Container.State.Running {
		t.Fatalf("container %s is not running", containerID)
	}

	// Теперь контейнер существует и находится в running.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	select {
	case err := <-finished:
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected nonzero CLI exit, got %v\n%s", err, output.String())
		}

		if exitErr.ExitCode() != 125 {
			t.Fatalf(
				"CLI exit code = %d; want 125\n%s",
				exitErr.ExitCode(),
				output.String(),
			)
		}

	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		<-finished
		t.Fatal("CLI did not stop after SIGINT")
	}

	// Контейнер должен быть удалён до возврата из CLI.
	inspectCtx, cancelInspect := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelInspect()

	_, inspectErr := dockerClient.ContainerInspect(
		inspectCtx,
		containerID,
		client.ContainerInspectOptions{},
	)
	if !errdefs.IsNotFound(inspectErr) {
		t.Fatalf(
			"expected container %s to be removed; inspect error: %v",
			containerID,
			inspectErr,
		)
	}

	t.Logf("SIGINT handled, container %s removed", containerID)
}
