package docker

import (
	"testing"

	"github.com/moby/moby/api/types/mount"
)

func TestSnapshotExecutionMountSecurity(t *testing.T) {
	volumes := &SnapshotVolumes{
		Workspace:   "test-workspace-volume",
		ModuleCache: "test-module-volume",
	}

	mounts := snapshotExecutionMounts(volumes)

	if len(mounts) != 2 {
		t.Fatalf("expected exactly 2 mounts, got %d", len(mounts))
	}

	expected := map[string]string{
		"/workspace":           volumes.Workspace,
		"/module-cache-source": volumes.ModuleCache,
	}

	seen := make(map[string]bool)

	for _, m := range mounts {
		if m.Type != mount.TypeVolume {
			t.Fatalf("non-volume mount found: %+v", m)
		}

		wantSource, ok := expected[m.Target]
		if !ok {
			t.Fatalf("unexpected mount target: %q", m.Target)
		}

		if seen[m.Target] {
			t.Fatalf("duplicate mount target: %q", m.Target)
		}
		seen[m.Target] = true

		if m.Source != wantSource {
			t.Fatalf(
				"mount %s source = %q, want %q",
				m.Target, m.Source, wantSource,
			)
		}

		if !m.ReadOnly {
			t.Fatalf("mount %s is writable", m.Target)
		}

		if m.VolumeOptions == nil || !m.VolumeOptions.NoCopy {
			t.Fatalf("mount %s allows volume copy-up", m.Target)
		}

		if m.BindOptions != nil {
			t.Fatalf("unexpected bind options for %s", m.Target)
		}
	}

	for target := range expected {
		if !seen[target] {
			t.Fatalf("missing mount target: %s", target)
		}
	}
}

func TestSnapshotExecutionContainerSecurity(t *testing.T) {
	volumes := &SnapshotVolumes{
		Workspace:   "test-workspace-volume",
		ModuleCache: "test-module-volume",
	}

	options := newSnapshotExecutionOptions(
		goReplayImage,
		"10001:10001",
		volumes,
	)

	config := options.Config
	host := options.HostConfig

	if config == nil || host == nil {
		t.Fatal("missing container configuration")
	}

	if config.User != "10001:10001" {
		t.Fatalf("unexpected container user: %q", config.User)
	}

	if config.WorkingDir != "/workspace" {
		t.Fatalf("unexpected working directory: %q", config.WorkingDir)
	}

	if config.Image != goReplayImage {
		t.Fatalf("unexpected Docker image: %q", config.Image)
	}

	if options.Platform == nil ||
		options.Platform.OS != "linux" ||
		options.Platform.Architecture != "amd64" {
		t.Fatalf("unexpected container platform: %+v", options.Platform)
	}

	if string(host.NetworkMode) != "none" {
		t.Fatalf("network enabled: %s", host.NetworkMode)
	}

	if !host.ReadonlyRootfs {
		t.Fatal("root filesystem is writable")
	}

	if host.Privileged {
		t.Fatal("privileged container")
	}

	if len(host.CapDrop) != 1 || host.CapDrop[0] != "ALL" {
		t.Fatalf("unexpected capabilities: %v", host.CapDrop)
	}

	if len(host.SecurityOpt) != 1 ||
		host.SecurityOpt[0] != "no-new-privileges" {
		t.Fatalf("unexpected security options: %v", host.SecurityOpt)
	}

	if host.PidsLimit == nil || *host.PidsLimit != 64 {
		t.Fatalf("unexpected PIDs limit: %v", host.PidsLimit)
	}

	if host.Memory != 256*1024*1024 {
		t.Fatalf("unexpected memory limit: %d", host.Memory)
	}

	if host.NanoCPUs != 1_000_000_000 {
		t.Fatalf("unexpected CPU limit: %d", host.NanoCPUs)
	}

	if len(host.Mounts) != 2 {
		t.Fatalf("expected two mounts, got %d", len(host.Mounts))
	}

	for _, m := range host.Mounts {
		if m.Type != mount.TypeVolume ||
			!m.ReadOnly ||
			m.VolumeOptions == nil ||
			!m.VolumeOptions.NoCopy {
			t.Fatalf("insecure snapshot mount: %+v", m)
		}
	}

	if len(host.Binds) != 0 {
		t.Fatalf("unexpected host binds: %v", host.Binds)
	}

	for _, expected := range []string{
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"CGO_ENABLED=0",
		"GOMODCACHE=/gomodcache",
		"GOCACHE=/gocache",
	} {
		found := false
		for _, value := range config.Env {
			if value == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing environment setting: %s", expected)
		}
	}

	for _, target := range []string{
		"/tmp",
		"/gocache",
		"/gomodcache",
	} {
		if host.Tmpfs[target] == "" {
			t.Fatalf("missing writable tmpfs: %s", target)
		}
	}
}
