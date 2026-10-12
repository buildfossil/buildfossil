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

func TestSnapshotVolumeCreateRollbackErrorsV3(t *testing.T) {
	for _, tc := range []struct {
		name          string
		inspectStatus int
		removeStatus  int
		wantInspect   int
		wantRemove    int
		wantError     string
	}{
		{
			name:          "lost create response owned volume",
			inspectStatus: http.StatusOK,
			removeStatus:  http.StatusNoContent,
			wantInspect:   1,
			wantRemove:    1,
		},
		{
			name:          "volume not found",
			inspectStatus: http.StatusNotFound,
			wantInspect:   1,
			wantRemove:    0,
		},
		{
			name:          "inspect failure",
			inspectStatus: http.StatusInternalServerError,
			wantInspect:   1,
			wantRemove:    0,
			wantError:     "inspect snapshot volume",
		},
		{
			name:          "remove failure",
			inspectStatus: http.StatusOK,
			removeStatus:  http.StatusInternalServerError,
			wantInspect:   1,
			wantRemove:    1,
			wantError:     "remove owned snapshot volume",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const volumeName = "buildfossil-snapshot-error-test"

			var mu sync.Mutex
			var labels map[string]string
			var createCalls, inspectCalls, removeCalls int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()

				w.Header().Set("Content-Type", "application/json")

				switch {
				case r.Method == http.MethodPost &&
					strings.HasSuffix(r.URL.Path, "/volumes/create"):

					createCalls++

					var request struct {
						Name   string            `json:"Name"`
						Labels map[string]string `json:"Labels"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Errorf("decode create: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if request.Name != volumeName {
						t.Errorf("unexpected volume name: %q", request.Name)
					}
					labels = request.Labels

					// Simulates Docker creating the volume while the API
					// returns an error instead of a successful response.
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprint(w, `{"message":"injected create response failure"}`)

				case r.Method == http.MethodGet &&
					strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):

					inspectCalls++

					if tc.inspectStatus != http.StatusOK {
						w.WriteHeader(tc.inspectStatus)
						_, _ = fmt.Fprint(w, `{"message":"injected inspect failure"}`)
						return
					}

					_ = json.NewEncoder(w).Encode(map[string]any{
						"Name":   volumeName,
						"Driver": "local",
						"Labels": labels,
					})

				case r.Method == http.MethodDelete &&
					strings.HasSuffix(r.URL.Path, "/volumes/"+volumeName):

					removeCalls++
					w.WriteHeader(tc.removeStatus)
					if tc.removeStatus != http.StatusNoContent {
						_, _ = fmt.Fprint(w, `{"message":"injected remove failure"}`)
					}

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

			volumes, err := createSnapshotVolumesFromReadersWithNameGenerator(
				context.Background(),
				dockerClient,
				"test-image",
				bytes.NewReader([]byte("workspace-tar")),
				bytes.NewReader([]byte("module-tar")),
				func() (string, error) { return volumeName, nil },
			)

			if volumes != nil {
				t.Fatal("unexpected volumes returned after create error")
			}
			if err == nil {
				t.Fatal("original creation error was lost")
			}
			if !strings.Contains(err.Error(), "injected create response failure") {
				t.Fatalf("creation error missing: %v", err)
			}
			if tc.wantError != "" &&
				!strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("cleanup error missing (%q): %v", tc.wantError, err)
			}

			mu.Lock()
			defer mu.Unlock()

			t.Logf(
				"DOCKER_API_COUNTS create=%d inspect=%d remove=%d",
				createCalls, inspectCalls, removeCalls,
			)

			if createCalls != 1 ||
				inspectCalls != tc.wantInspect ||
				removeCalls != tc.wantRemove {
				t.Fatalf(
					"unexpected calls: create=%d inspect=%d remove=%d",
					createCalls, inspectCalls, removeCalls,
				)
			}

			if labels["org.buildfossil.component"] != "snapshot" ||
				labels["org.buildfossil.kind"] != "workspace" ||
				labels["org.buildfossil.ownership-token"] == "" {
				t.Fatalf("missing volume ownership identity: %v", labels)
			}
		})
	}
}
