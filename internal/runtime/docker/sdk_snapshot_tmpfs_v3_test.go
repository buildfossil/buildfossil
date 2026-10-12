package docker

import (
	"testing"
)

func TestSnapshotExecutionTmpfsLimitsV3(t *testing.T) {
	options := newSnapshotExecutionOptions(
		goReplayImage,
		"10001:10001",
		&SnapshotVolumes{},
	)

	want := map[string]string{
		"/tmp":        "rw,nosuid,nodev,size=512m,mode=1777",
		"/gocache":    "rw,nosuid,nodev,size=256m,mode=1777",
		"/gomodcache": "rw,nosuid,nodev,size=256m,mode=1777",
	}

	host := options.HostConfig
	if host == nil {
		t.Fatal("missing Docker HostConfig")
	}

	if len(host.Tmpfs) != len(want) {
		t.Fatalf(
			"tmpfs mount count = %d, want %d",
			len(host.Tmpfs), len(want),
		)
	}

	for target, expected := range want {
		if got := host.Tmpfs[target]; got != expected {
			t.Errorf(
				"tmpfs %s = %q, want %q",
				target, got, expected,
			)
		}
	}

	if string(host.NetworkMode) != "none" {
		t.Fatal("network must remain disabled")
	}

	if !host.ReadonlyRootfs {
		t.Fatal("root filesystem must remain read-only")
	}

	if host.Privileged {
		t.Fatal("privileged container is forbidden")
	}

	if len(host.CapDrop) != 1 || host.CapDrop[0] != "ALL" {
		t.Fatalf("unexpected capabilities: %v", host.CapDrop)
	}

	if len(host.SecurityOpt) != 1 ||
		host.SecurityOpt[0] != "no-new-privileges" {
		t.Fatalf("unexpected security options: %v", host.SecurityOpt)
	}

	if host.PidsLimit == nil || *host.PidsLimit != 64 {
		t.Fatalf("unexpected PID limit: %v", host.PidsLimit)
	}

	if host.Memory != 256<<20 {
		t.Fatalf("unexpected default memory limit: %d", host.Memory)
	}

	for _, memory := range []int64{
		256 << 20,
		512 << 20,
		1 << 30,
		2 << 30,
	} {
		t.Run(memoryLabelV3(memory), func(t *testing.T) {
			copyOptions := newSnapshotExecutionOptions(
				goReplayImage,
				"10001:10001",
				&SnapshotVolumes{},
			)

			if err := applyReplayMemoryV3(&copyOptions, memory); err != nil {
				t.Fatal(err)
			}

			if copyOptions.HostConfig.Memory != memory {
				t.Fatalf(
					"memory = %d, want %d",
					copyOptions.HostConfig.Memory, memory,
				)
			}

			if !copyOptions.HostConfig.ReadonlyRootfs ||
				string(copyOptions.HostConfig.NetworkMode) != "none" {
				t.Fatal("memory override weakened isolation")
			}
		})
	}
}

func memoryLabelV3(memory int64) string {
	switch memory {
	case 256 << 20:
		return "256MiB"
	case 512 << 20:
		return "512MiB"
	case 1 << 30:
		return "1GiB"
	case 2 << 30:
		return "2GiB"
	default:
		return "unknown"
	}
}
