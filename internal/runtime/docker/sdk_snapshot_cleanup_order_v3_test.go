package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotCloseDefersVolumeRemovalAfterContainerFailureV3(t *testing.T) {
	const (
		containerID = "buildfossil-preparation-cleanup-test"
		volumeName  = "buildfossil-snapshot-cleanup-test"
		token       = "expected-volume-ownership-token"
	)

	var mu sync.Mutex
	var events []string
	removeFailures := 1

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()

			switch {
			case r.Method == http.MethodDelete &&
				strings.HasSuffix(r.URL.Path, "/containers/"+containerID):
				events = append(events, "container-remove")

				if removeFailures > 0 {
					removeFailures--
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprint(w, `{"message":"temporary container removal failure"}`)
					return
				}

				w.WriteHeader(http.StatusNoContent)

			case r.Method == http.MethodGet &&
				strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):
				events = append(events, "volume-inspect")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Name": volumeName,
					"Labels": map[string]string{
						"org.buildfossil.component":       "snapshot",
						"org.buildfossil.kind":            "workspace",
						"org.buildfossil.ownership-token": token,
					},
				})

			case r.Method == http.MethodDelete &&
				strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):
				events = append(events, "volume-remove")
				w.WriteHeader(http.StatusNoContent)

			default:
				t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}
		},
	))
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

	volumes := &SnapshotVolumes{
		client:       dockerClient,
		Workspace:    volumeName,
		containers:   []string{containerID},
		volumeTokens: map[string]string{volumeName: token},
		volumeKinds:  map[string]string{volumeName: "workspace"},
	}

	firstErr := volumes.Close()
	if firstErr == nil ||
		!strings.Contains(firstErr.Error(), "temporary container removal failure") {
		t.Fatalf("missing container cleanup error: %v", firstErr)
	}

	mu.Lock()
	firstEvents := append([]string(nil), events...)
	mu.Unlock()

	t.Logf("FIRST_CLOSE_EVENTS=%v", firstEvents)

	if len(firstEvents) != 1 || firstEvents[0] != "container-remove" {
		t.Fatalf(
			"BUG: volume cleanup started after container removal failure: %v",
			firstEvents,
		)
	}

	if volumes.Workspace != volumeName || len(volumes.containers) != 1 {
		t.Fatalf("cleanup state discarded after failure: %+v", volumes)
	}

	if err := volumes.Close(); err != nil {
		t.Fatalf("retry Close: %v", err)
	}

	mu.Lock()
	finalEvents := append([]string(nil), events...)
	mu.Unlock()

	want := []string{
		"container-remove",
		"container-remove",
		"volume-inspect",
		"volume-remove",
	}

	if len(finalEvents) != len(want) {
		t.Fatalf("events=%v, want=%v", finalEvents, want)
	}

	for i := range want {
		if finalEvents[i] != want[i] {
			t.Fatalf("events=%v, want=%v", finalEvents, want)
		}
	}

	if volumes.Workspace != "" || len(volumes.containers) != 0 {
		t.Fatalf("cleanup state remains after successful retry: %+v", volumes)
	}

	t.Logf("RETRY_EVENTS=%v", finalEvents)
}

// A lost ContainerRemove response may leave the caller uncertain even
// though Docker already removed the preparation container. A subsequent
// 404 for its recorded ID must not permanently block volume cleanup.
func TestSnapshotCloseHandlesAlreadyRemovedPreparationContainerV3(t *testing.T) {
	const (
		containerID = "buildfossil-preparation-already-removed-test"
		volumeName  = "buildfossil-snapshot-already-removed-test"
		token       = "expected-already-removed-token"
	)

	var mu sync.Mutex
	var events []string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()

			switch {
			case r.Method == http.MethodDelete &&
				strings.HasSuffix(r.URL.Path, "/containers/"+containerID):
				events = append(events, "container-remove-not-found")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"No such container"}`)

			case r.Method == http.MethodGet &&
				strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):
				events = append(events, "volume-inspect")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Name": volumeName,
					"Labels": map[string]string{
						"org.buildfossil.component":       "snapshot",
						"org.buildfossil.kind":            "workspace",
						"org.buildfossil.ownership-token": token,
					},
				})

			case r.Method == http.MethodDelete &&
				strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):
				events = append(events, "volume-remove")
				w.WriteHeader(http.StatusNoContent)

			default:
				t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}
		},
	))
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

	volumes := &SnapshotVolumes{
		client:       dockerClient,
		Workspace:    volumeName,
		containers:   []string{containerID},
		volumeTokens: map[string]string{volumeName: token},
		volumeKinds:  map[string]string{volumeName: "workspace"},
	}

	if err := volumes.Close(); err != nil {
		t.Fatalf("already-removed preparation container blocked cleanup: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()

	want := []string{
		"container-remove-not-found",
		"volume-inspect",
		"volume-remove",
	}

	if len(got) != len(want) {
		t.Fatalf("cleanup operations=%v, want=%v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cleanup operations=%v, want=%v", got, want)
		}
	}

	if volumes.Workspace != "" || len(volumes.containers) != 0 {
		t.Fatalf("cleanup state remains: %+v", volumes)
	}

	if err := volumes.Close(); err != nil {
		t.Fatalf("repeated Close: %v", err)
	}

	t.Logf("ALREADY_REMOVED_EVENTS=%v", got)
}
