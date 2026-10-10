package docker

import "testing"

func TestSnapshotVolumesCloseNil(t *testing.T) {
	var volumes *SnapshotVolumes

	if err := volumes.Close(); err != nil {
		t.Fatalf("close nil manager: %v", err)
	}
}

func TestSnapshotVolumesCloseWithoutResources(t *testing.T) {
	volumes := &SnapshotVolumes{}

	if err := volumes.Close(); err != nil {
		t.Fatalf("close empty manager: %v", err)
	}
}
