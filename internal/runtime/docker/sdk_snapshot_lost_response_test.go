package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotPreparationContainerLostCreateResponse(t *testing.T) {
	var mu sync.Mutex
	containerExists := false
	containerRemoved := false
	containerInspected := false
	volumeRemoved := false
	volumeInspected := false
	var volumeLabels map[string]string
	containerName := ""
	ownershipToken := ""

	const containerID = "buildfossil-test-container-id"

	const volumeName = "buildfossil-snapshot-lost-response-test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/volumes/create"):
			var request struct {
				Name   string            `json:"Name"`
				Labels map[string]string `json:"Labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode volume create: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.Name != volumeName {
				t.Errorf("unexpected volume name: %q", request.Name)
			}
			volumeLabels = request.Labels
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"Name":%q,"Driver":"local"}`, volumeName)

		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):
			volumeInspected = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Name": volumeName, "Driver": "local", "Labels": volumeLabels,
			})

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create"):
			containerExists = true
			containerName = r.URL.Query().Get("name")

			var request struct {
				Labels map[string]string `json:"Labels"`
			}
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("read container create request: %v", readErr)
				return
			}

			if err := json.Unmarshal(body, &request); err != nil {
				t.Errorf("decode container create request: %v", err)
				return
			}
			ownershipToken = request.Labels["org.buildfossil.ownership-token"]

			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("test HTTP writer does not support hijacking")
				return
			}

			conn, _, hijackErr := hijacker.Hijack()
			if hijackErr != nil {
				t.Errorf("hijack simulated Docker connection: %v", hijackErr)
				return
			}
			_ = conn.Close()
			return

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"):
			if !containerExists || containerName == "" ||
				!strings.Contains(r.URL.Path, "/containers/"+containerName+"/json") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			containerInspected = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(
				w,
				`{"Id":%q,"Name":%q,"Config":{"Labels":{"org.buildfossil.component":"replay","org.buildfossil.ownership-token":%q}}}`,
				containerID, "/"+containerName, ownershipToken,
			)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/"):
			if !containerInspected ||
				!strings.Contains(r.URL.Path, "/containers/"+containerID) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			containerExists = false
			containerRemoved = true
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/volumes/"):
			if !volumeInspected ||
				!strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			volumeRemoved = true
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(
				w,
				`{"message":"unexpected request: %s %s"}`,
				r.Method,
				r.URL.Path,
			)
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
		func() (string, error) { return volumeName, nil },
	)

	if volumes != nil {
		t.Fatal("unexpected snapshot manager returned after create error")
	}
	if err == nil {
		t.Fatal("expected Docker API error")
	}

	mu.Lock()
	defer mu.Unlock()

	if !volumeRemoved {
		t.Fatal("snapshot volume rollback was not attempted")
	}
	if containerExists || !containerRemoved {
		t.Fatal("BUG: preparation container survived lost ContainerCreate response")
	}
	if !containerInspected {
		t.Fatal("container ownership was not inspected before removal")
	}
	if containerName == "" || ownershipToken == "" {
		t.Fatal("container name or ownership token was not set before creation")
	}
}
