package docker

import (
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

func TestSnapshotExecutionContainerLostCreateResponse(t *testing.T) {
	const containerID = "buildfossil-execution-lost-response-test-id"

	var mu sync.Mutex
	var containerExists bool
	var containerInspected bool
	var containerRemoved bool
	var removalOptionsVerified bool
	var containerName string
	var ownershipToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodPost &&
			strings.HasSuffix(r.URL.Path, "/containers/create"):

			containerExists = true
			containerName = r.URL.Query().Get("name")

			var request struct {
				Labels map[string]string `json:"Labels"`
			}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create request: %v", err)
				return
			}

			ownershipToken = request.Labels["org.buildfossil.ownership-token"]

			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("HTTP hijacking unavailable")
				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack Docker response: %v", err)
				return
			}

			_ = conn.Close()

		case r.Method == http.MethodGet &&
			strings.Contains(r.URL.Path, "/containers/"):

			if !containerExists || containerName == "" ||
				!strings.HasSuffix(
					r.URL.Path,
					"/containers/"+containerName+"/json",
				) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			containerInspected = true
			w.Header().Set("Content-Type", "application/json")

			_, _ = fmt.Fprintf(
				w,
				`{"Id":%q,"Name":%q,"Config":{"Labels":{"org.buildfossil.component":"replay","org.buildfossil.ownership-token":%q}}}`,
				containerID,
				"/"+containerName,
				ownershipToken,
			)

		case r.Method == http.MethodDelete &&
			strings.Contains(r.URL.Path, "/containers/"):

			if !containerInspected ||
				!strings.HasSuffix(r.URL.Path, "/containers/"+containerID) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if r.URL.Query().Get("force") != "1" ||
				r.URL.Query().Get("v") != "1" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprint(w, `{"message":"expected force=1 and v=1"}`)
				return
			}

			removalOptionsVerified = true
			containerExists = false
			containerRemoved = true
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
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

	options := newSnapshotExecutionOptions(
		"test-image",
		"10001:10001",
		&SnapshotVolumes{
			Workspace:   "test-workspace-volume",
			ModuleCache: "test-module-volume",
		},
	)

	created, err := createSnapshotExecutionContainer(
		context.Background(),
		dockerClient,
		options,
	)

	if err == nil {
		t.Fatal("expected Docker ContainerCreate connection error")
	}
	if created.ID != "" {
		t.Fatalf("unexpected container ID after create failure: %q", created.ID)
	}

	mu.Lock()
	defer mu.Unlock()

	if containerName == "" || ownershipToken == "" {
		t.Fatal("missing execution container identity")
	}
	if !containerInspected {
		t.Fatal("execution container was not inspected after lost response")
	}
	if containerExists || !containerRemoved {
		t.Fatal("BUG: execution container survived lost ContainerCreate response")
	}
	if !removalOptionsVerified {
		t.Fatal("execution container was not removed with force=1 and v=1")
	}
}
