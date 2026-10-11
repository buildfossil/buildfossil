package docker

import "testing"

func TestApplyReplayMemoryV3(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  int64
		valid bool
	}{
		{"default", 0, 256 << 20, true},
		{"256 MiB", 256 << 20, 256 << 20, true},
		{"512 MiB", 512 << 20, 512 << 20, true},
		{"1 GiB", 1 << 30, 1 << 30, true},
		{"2 GiB", 2 << 30, 2 << 30, true},
		{"negative", -1, 0, false},
		{"128 MiB", 128 << 20, 0, false},
		{"3 GiB", 3 << 30, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := newSecureContainerOptions(
				"/tmp/workspace",
				[]string{"/bin/true"},
				"10001:10001",
			)

			originalNetwork := options.HostConfig.NetworkMode
			originalCPU := options.HostConfig.Resources.NanoCPUs
			originalPIDs := *options.HostConfig.Resources.PidsLimit
			originalReadonly := options.HostConfig.ReadonlyRootfs

			err := applyReplayMemoryV3(&options, tt.value)

			if (err == nil) != tt.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}

			if tt.valid && options.HostConfig.Resources.Memory != tt.want {
				t.Fatalf("memory = %d; want %d",
					options.HostConfig.Resources.Memory, tt.want)
			}

			if options.HostConfig.NetworkMode != originalNetwork ||
				options.HostConfig.Resources.NanoCPUs != originalCPU ||
				*options.HostConfig.Resources.PidsLimit != originalPIDs ||
				options.HostConfig.ReadonlyRootfs != originalReadonly {
				t.Fatal("memory override changed another security setting")
			}
		})
	}
}
