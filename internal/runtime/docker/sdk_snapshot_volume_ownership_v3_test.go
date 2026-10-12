package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotVolumeRollbackPreservesForeignVolumeV3(t *testing.T) {
	const volumeName = "buildfossil-snapshot-foreign-volume-test"

	var mu sync.Mutex
	var createCalls int
	var inspectCalls int
	var removeCalls int
	var requestedToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodPost &&
			strings.HasSuffix(r.URL.Path, "/volumes/create"):

			createCalls++

			var request struct {
				Name   string            `json:"Name"`
				Labels map[string]string `json:"Labels"`
			}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode VolumeCreate request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if request.Name != volumeName {
				t.Errorf("unexpected volume name: %q", request.Name)
			}

			requestedToken = request.Labels["org.buildfossil.ownership-token"]

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"message":"volume creation failed"}`)

		case r.Method == http.MethodGet &&
			strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):

			inspectCalls++

			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(
				w,
				`{"Name":%q,"Driver":"local","Labels":{"org.buildfossil.component":"snapshot","org.buildfossil.kind":"workspace","org.buildfossil.ownership-token":"foreign-token"}}`,
				volumeName,
			)

		case r.Method == http.MethodDelete &&
			strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):

			removeCalls++
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"message":"unexpected Docker request"}`)
		}
	}))
	defer server.Close()

	dockerClient, err := client.New(
		client.WithHost("tcp://"+strings.TrimPrefix(server.URL, "http://")),
		client.WithHTTPClient(server.Client()),
		client.WithAPIVersion("1.44"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.Close()

	volumes, err := createSnapshotVolumesFromReadersWithNameGenerator(
		context.Background(),
		dockerClient,
		"test-image",
		bytes.NewReader([]byte("workspace-tar")),
		bytes.NewReader([]byte("module-tar")),
		func() (string, error) {
			return volumeName, nil
		},
	)

	if volumes != nil {
		t.Fatal("unexpected snapshot volumes on failed creation")
	}
	if err == nil || !strings.Contains(err.Error(), "volume creation failed") {
		t.Fatalf("original VolumeCreate error lost: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if createCalls != 1 {
		t.Fatalf("VolumeCreate calls = %d, want 1", createCalls)
	}

	t.Logf(
		"DOCKER_API_COUNTS create=%d inspect=%d remove=%d",
		createCalls, inspectCalls, removeCalls,
	)

	if removeCalls != 0 {
		t.Errorf(
			"SECURITY FAILURE: foreign volume removed %d times",
			removeCalls,
		)
	}

	if inspectCalls != 1 {
		t.Errorf("VolumeInspect calls = %d, want 1", inspectCalls)
	}

	if requestedToken == "" || requestedToken == "foreign-token" {
		t.Errorf("missing or invalid ownership token: %q", requestedToken)
	}
}
