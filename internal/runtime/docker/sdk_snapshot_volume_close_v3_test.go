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

func TestSnapshotVolumeCloseOwnershipV3(t *testing.T) {
	for _, tc := range []struct {
		name       string
		foreign    bool
		wantRemove int
		wantError  bool
	}{
		{name: "owned volume and repeated close", wantRemove: 1},
		{name: "reused name with foreign token", foreign: true, wantRemove: 0, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const name = "rew-snapshot-close-test"
			const token = "original-owner-token"

			var mu sync.Mutex
			var inspectCalls, removeCalls int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()

				w.Header().Set("Content-Type", "application/json")

				switch {
				case r.Method == http.MethodGet &&
					strings.HasSuffix(r.URL.Path, "/volumes/"+name):
					inspectCalls++

					currentToken := token
					if tc.foreign {
						currentToken = "replacement-owner-token"
					}

					_ = json.NewEncoder(w).Encode(map[string]any{
						"Name":   name,
						"Driver": "local",
						"Labels": map[string]string{
							"org.rew.component":       "snapshot",
							"org.rew.kind":            "workspace",
							"org.rew.ownership-token": currentToken,
						},
					})

				case r.Method == http.MethodDelete &&
					strings.HasSuffix(r.URL.Path, "/volumes/"+name):
					removeCalls++
					w.WriteHeader(http.StatusNoContent)

				default:
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
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

			volumes := &SnapshotVolumes{
				client:       dockerClient,
				Workspace:    name,
				volumeTokens: map[string]string{name: token},
				volumeKinds:  map[string]string{name: "workspace"},
			}

			firstErr := volumes.Close()
			if tc.wantError {
				if firstErr == nil ||
					!strings.Contains(firstErr.Error(), "unverified snapshot volume ownership") {
					t.Fatalf("expected ownership rejection: %v", firstErr)
				}
			} else if firstErr != nil {
				t.Fatalf("close owned volume: %v", firstErr)
			}

			if !tc.foreign {
				if volumes.Workspace != "" {
					t.Fatalf("owned volume remained registered: %q", volumes.Workspace)
				}
				if err := volumes.Close(); err != nil {
					t.Fatalf("repeated Close: %v", err)
				}
			}

			mu.Lock()
			defer mu.Unlock()

			t.Logf(
				"DOCKER_API_COUNTS inspect=%d remove=%d",
				inspectCalls, removeCalls,
			)

			if inspectCalls != 1 || removeCalls != tc.wantRemove {
				t.Fatalf(
					"unexpected API calls: inspect=%d remove=%d",
					inspectCalls, removeCalls,
				)
			}
		})
	}
}
