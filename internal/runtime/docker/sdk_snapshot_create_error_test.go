package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotVolumeContainerCreateNotFound(t *testing.T) {
	var mu sync.Mutex
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/volumes/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Name":"test-snapshot-volume","Driver":"local"}`)

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"No such image: test-image"}`)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/volumes/"):
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"message":"unexpected request: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	dockerClient, err := client.NewClientWithOpts(
		client.WithHost("tcp://"+strings.TrimPrefix(server.URL, "http://")),
		client.WithHTTPClient(server.Client()),
		client.WithVersion("1.44"),
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
			return "test-snapshot-volume", nil
		},
	)

	if volumes != nil {
		t.Fatal("unexpected snapshot volumes returned after ContainerCreate failure")
	}
	if err == nil {
		t.Fatal("ContainerCreate NotFound error was ignored")
	}
	if !strings.Contains(err.Error(), "create workspace preparation container") {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	t.Logf("Docker API requests: %v", requests)

	var created, copied, cleaned bool
	for _, request := range requests {
		if strings.Contains(request, "/containers/create") {
			created = true
		}
		if strings.Contains(request, "/archive") {
			copied = true
		}
		if strings.HasPrefix(request, "DELETE ") && strings.Contains(request, "/volumes/") {
			cleaned = true
		}
	}

	if !created {
		t.Fatal("ContainerCreate was not attempted")
	}
	if copied {
		t.Fatal("TAR upload attempted after ContainerCreate failure")
	}
	if !cleaned {
		t.Fatal("snapshot volume cleanup was not attempted")
	}
}
