package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigureGoModuleArtifacts(t *testing.T) {
	artifactsDir := t.TempDir()

	runtime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	options := newSecureContainerOptions(
		t.TempDir(),
		[]string{"go", "version"},
		"65534:65534",
	)

	err := configureGoModuleArtifacts(
		&options,
		runtime,
		artifactsDir,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(options.HostConfig.Mounts) != 2 {
		t.Fatalf(
			"mount count = %d; want 2",
			len(options.HostConfig.Mounts),
		)
	}

	moduleMount := options.HostConfig.Mounts[1]

	if moduleMount.Target != "/module-artifacts" ||
		!moduleMount.ReadOnly ||
		moduleMount.Source != artifactsDir {
		t.Fatalf("invalid module mount: %+v", moduleMount)
	}

	if got := options.HostConfig.Tmpfs["/gomodcache"]; got == "" {
		t.Fatal("missing module cache tmpfs")
	}

	foundEnv := false
	for _, value := range options.Config.Env {
		if value == "GOMODCACHE=/gomodcache" {
			foundEnv = true
		}
	}

	if !foundEnv {
		t.Fatal("missing GOMODCACHE environment variable")
	}

	if string(options.HostConfig.NetworkMode) != "none" {
		t.Fatal("network isolation changed")
	}

	if !options.HostConfig.ReadonlyRootfs {
		t.Fatal("read-only root filesystem changed")
	}

	if options.HostConfig.Privileged {
		t.Fatal("container must not be privileged")
	}

	if len(options.HostConfig.CapDrop) != 1 ||
		options.HostConfig.CapDrop[0] != "ALL" {
		t.Fatalf("capabilities changed: %v", options.HostConfig.CapDrop)
	}

	if len(options.HostConfig.SecurityOpt) != 1 ||
		options.HostConfig.SecurityOpt[0] != "no-new-privileges" {
		t.Fatalf("security options changed: %v", options.HostConfig.SecurityOpt)
	}

	if options.HostConfig.Memory != 256*1024*1024 {
		t.Fatalf("memory limit changed: %d", options.HostConfig.Memory)
	}

	if options.HostConfig.PidsLimit == nil ||
		*options.HostConfig.PidsLimit != 64 {
		t.Fatalf("PID limit changed: %v", options.HostConfig.PidsLimit)
	}

	if options.HostConfig.NanoCPUs != 1_000_000_000 {
		t.Fatalf("CPU limit changed: %d", options.HostConfig.NanoCPUs)
	}

	workspaceMount := options.HostConfig.Mounts[0]

	if workspaceMount.Target != "/workspace" ||
		!workspaceMount.ReadOnly {
		t.Fatalf("workspace security changed: %+v", workspaceMount)
	}

	if options.HostConfig.Tmpfs["/gomodcache"] !=
		"rw,nosuid,nodev,size=64m,mode=1777" {
		t.Fatal("module cache tmpfs configuration changed")
	}
}

func TestConfigureGoModuleArtifactsRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	validRuntime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	tests := []struct {
		name    string
		runtime *RuntimeSpec
		dir     string
	}{
		{"missing runtime", nil, dir},
		{"unsupported runtime", &RuntimeSpec{Kind: "go", Version: "1.25.0"}, dir},
		{"empty directory", validRuntime, ""},
		{"regular file", validRuntime, file},
		{"missing directory", validRuntime, filepath.Join(dir, "missing")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := newSecureContainerOptions(
				t.TempDir(),
				[]string{"go", "version"},
				"65534:65534",
			)

			if err := configureGoModuleArtifacts(
				&options,
				tt.runtime,
				tt.dir,
			); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
