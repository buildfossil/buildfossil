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

func TestStandardExecutionContainerLostCreateResponse(t *testing.T) {
	const containerID = "rew-standard-execution-test-id"
	const runID = "0123456789abcdef0123456789abcdef"

	var mu sync.Mutex
	var containerExists bool
	var containerInspected bool
	var containerRemoved bool
	var containerName string
	var ownershipToken string
	var observedRunID string

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

			ownershipToken = request.Labels["org.rew.ownership-token"]
			observedRunID = request.Labels["org.rew.run-id"]

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
				`{"Id":%q,"Name":%q,"Config":{"Labels":{"org.rew.component":"replay","org.rew.ownership-token":%q}}}`,
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

	options := newSecureContainerOptions(
		"/test-workspace",
		[]string{"/bin/sh", "-c", "true"},
		"10001:10001",
	)
	options.Config.Labels["org.rew.run-id"] = runID

	created, err := createStandardExecutionContainer(
		context.Background(),
		dockerClient,
		options,
		runID,
	)

	if err == nil {
		t.Fatal("expected lost ContainerCreate response error")
	}
	if created.ID != "" {
		t.Fatalf("unexpected container ID: %q", created.ID)
	}

	mu.Lock()
	defer mu.Unlock()

	if observedRunID != runID {
		t.Fatalf("run ID label changed: %q", observedRunID)
	}
	if containerName != "rew-replay-exec-"+runID {
		t.Fatalf("unexpected execution container name: %q", containerName)
	}
	if ownershipToken != runID {
		t.Fatalf("ownership token mismatch: %q", ownershipToken)
	}
	if !containerInspected {
		t.Fatal("container ownership was not inspected")
	}
	if containerExists || !containerRemoved {
		t.Fatal("BUG: standard execution container survived lost response")
	}
}
