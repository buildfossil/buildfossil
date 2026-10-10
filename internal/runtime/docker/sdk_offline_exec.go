package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func RunWithSDKRuntimeOfflineExec(
	ctx context.Context,
	workspace string,
	argv []string,
	user string,
	stdout, stderr io.Writer,
	runtime *RuntimeSpec,
	cacheDir string,
) (exitCode int, runErr error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("docker SDK: empty command")
	}

	if runtime == nil ||
		runtime.Kind != "go" ||
		runtime.Version != "1.26.6" {
		return 0, fmt.Errorf("docker SDK: unsupported offline Go runtime")
	}

	parts := strings.Split(user, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("docker SDK: expected numeric UID:GID")
	}

	uid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || uid == 0 {
		return 0, fmt.Errorf("docker SDK: invalid or root UID")
	}

	if _, err := strconv.ParseUint(parts[1], 10, 32); err != nil {
		return 0, fmt.Errorf("docker SDK: invalid GID")
	}

	for _, path := range []string{workspace, cacheDir} {
		info, err := os.Lstat(path)
		if err != nil {
			return 0, fmt.Errorf("docker SDK: inspect staging path: %w", err)
		}

		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("docker SDK: invalid staging directory")
		}
	}

	image, err := selectReplayImage(runtime)
	if err != nil {
		return 0, err
	}

	dockerClient, err := NewSDKClient()
	if err != nil {
		return 0, err
	}
	defer dockerClient.Close()

	prepareCtx, cancelPrepare := context.WithTimeout(
		ctx,
		120*time.Second,
	)
	prepareErr := ensureReplayImage(
		prepareCtx, dockerClient, image,
	)
	cancelPrepare()

	if prepareErr != nil {
		return 0, fmt.Errorf("docker SDK: prepare image: %w", prepareErr)
	}

	execCtx, cancelExec := context.WithTimeout(
		ctx,
		goModulesExecutionTimeout,
	)
	defer cancelExec()

	options := newSecureContainerOptions(
		workspace,
		[]string{"/bin/sh", "-c", "while :; do sleep 60; done"},
		user,
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

	options.HostConfig.Mounts = append(
		options.HostConfig.Mounts,
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   cacheDir,
			Target:   "/module-cache-source",
			ReadOnly: true,
		},
	)

	created, err := dockerClient.ContainerCreate(execCtx, options)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: create offline container: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		_, cleanupErr := dockerClient.ContainerRemove(
			cleanupCtx,
			created.ID,
			client.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			},
		)

		if cleanupErr != nil {
			runErr = errors.Join(
				runErr,
				fmt.Errorf("docker SDK: cleanup offline container: %w", cleanupErr),
			)
		}
	}()

	if _, err := dockerClient.ContainerStart(
		execCtx,
		created.ID,
		client.ContainerStartOptions{},
	); err != nil {
		return 0, fmt.Errorf("docker SDK: start offline container: %w", err)
	}

	prepareCode, _, prepareStderr, err := executeInContainer(
		execCtx,
		dockerClient,
		created.ID,
		user,
		[]string{
			"/bin/sh",
			"-ec",
			"cp -R /module-cache-source/. /gomodcache/",
		},
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: prepare module cache: %w", err)
	}

	if prepareCode != 0 {
		return 0, fmt.Errorf(
			"docker SDK: module cache preparation failed (exit %d): %s",
			prepareCode,
			string(prepareStderr),
		)
	}

	// Materialize verified modules before executing the captured command.
	// Go may print "go: downloading ..." while extracting a ZIP from
	// the local download cache. Keep that preparation output separate
	// from the captured command's stderr.
	moduleCode, _, moduleStderr, err := executeInContainer(
		execCtx,
		dockerClient,
		created.ID,
		user,
		[]string{"go", "mod", "download"},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"docker SDK: offline module materialization: %w",
			err,
		)
	}

	if moduleCode != 0 {
		return 0, fmt.Errorf(
			"docker SDK: offline module materialization failed (exit %d): %s",
			moduleCode,
			string(moduleStderr),
		)
	}

	if err := execCtx.Err(); err != nil {
		return 0, fmt.Errorf(
			"docker SDK: offline module preparation canceled: %w",
			err,
		)
	}

	code, out, errOut, err := executeInContainer(
		execCtx,
		dockerClient,
		created.ID,
		user,
		argv,
	)
	if err != nil {
		return 0, fmt.Errorf("docker SDK: offline execution: %w", err)
	}

	if stdout != nil {
		if _, err := stdout.Write(out); err != nil {
			return 0, fmt.Errorf("docker SDK: stdout: %w", err)
		}
	}

	if stderr != nil {
		if _, err := stderr.Write(errOut); err != nil {
			return 0, fmt.Errorf("docker SDK: stderr: %w", err)
		}
	}

	return code, nil
}
