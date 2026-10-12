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

func TestOfflineExecutionContainerLostCreateResponse(t *testing.T) {
	const containerID = "buildfossil-offline-exec-test-id"

	var mu sync.Mutex
	var containerExists bool
	var inspected bool
	var removed bool
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
				!strings.HasSuffix(r.URL.Path, "/containers/"+containerName+"/json") {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			inspected = true
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

			if !inspected ||
				!strings.HasSuffix(r.URL.Path, "/containers/"+containerID) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if r.URL.Query().Get("force") != "1" ||
				r.URL.Query().Get("v") != "1" {
				t.Errorf("incorrect remove options: %s", r.URL.RawQuery)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			containerExists = false
			removed = true
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
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

	created, err := createOfflineExecutionContainer(
		context.Background(),
		dockerClient,
		options,
	)

	if err == nil {
		t.Fatal("expected lost ContainerCreate response error")
	}
	if created.ID != "" {
		t.Fatalf("unexpected container ID: %q", created.ID)
	}

	mu.Lock()
	defer mu.Unlock()

	if !strings.HasPrefix(containerName, "buildfossil-offline-exec-") {
		t.Fatalf("missing offline container identity: %q", containerName)
	}

	if len(ownershipToken) != 32 ||
		containerName != "buildfossil-offline-exec-"+ownershipToken {
		t.Fatalf(
			"invalid offline ownership identity: name=%q token=%q",
			containerName,
			ownershipToken,
		)
	}

	if !inspected {
		t.Fatal("container ownership was not inspected")
	}

	if containerExists || !removed {
		t.Fatal("BUG: offline execution container survived lost response")
	}
}
