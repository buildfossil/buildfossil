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

func TestStandardExecutionCreateErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		inspectStatus int
		foreignToken  bool
		removeStatus  int
		wantError     string
		wantRemove    bool
	}{
		{
			name:          "container not found",
			inspectStatus: http.StatusNotFound,
			wantError:     "create failed",
		},
		{
			name:          "inspect failure",
			inspectStatus: http.StatusInternalServerError,
			wantError:     "inspect uncertain standard execution container",
		},
		{
			name:          "foreign ownership",
			inspectStatus: http.StatusOK,
			foreignToken:  true,
			wantError:     "unverified ownership",
		},
		{
			name:          "remove failure",
			inspectStatus: http.StatusOK,
			removeStatus:  http.StatusInternalServerError,
			wantError:     "remove uncertain standard execution container",
			wantRemove:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const runID = "0123456789abcdef0123456789abcdef"
			const containerID = "standard-owned-test-container"

			var mu sync.Mutex
			var createCalls, inspectCalls, removeCalls int
			var observedName, observedRunID, observedToken string

			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()

					switch {
					case r.Method == http.MethodPost &&
						strings.HasSuffix(r.URL.Path, "/containers/create"):

						createCalls++
						observedName = r.URL.Query().Get("name")

						var request struct {
							Labels map[string]string `json:"Labels"`
						}

						if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
							t.Errorf("decode create request: %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}

						observedRunID = request.Labels["org.rew.run-id"]
						observedToken = request.Labels["org.rew.ownership-token"]

						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = fmt.Fprint(w, `{"message":"create failed"}`)

					case r.Method == http.MethodGet &&
						strings.Contains(r.URL.Path, "/containers/"):

						inspectCalls++

						if observedName == "" ||
							!strings.HasSuffix(r.URL.Path, "/containers/"+observedName+"/json") {
							t.Errorf("unexpected inspect target: %s", r.URL.Path)
							w.WriteHeader(http.StatusNotFound)
							return
						}

						if tc.inspectStatus != http.StatusOK {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(tc.inspectStatus)
							_, _ = fmt.Fprint(w, `{"message":"inspect failed"}`)
							return
						}

						token := observedToken
						if tc.foreignToken {
							token = "foreign-token"
						}

						w.Header().Set("Content-Type", "application/json")
						_, _ = fmt.Fprintf(
							w,
							`{"Id":%q,"Name":%q,"Config":{"Labels":{"org.rew.component":"replay","org.rew.ownership-token":%q}}}`,
							containerID,
							"/"+observedName,
							token,
						)

					case r.Method == http.MethodDelete &&
						strings.Contains(r.URL.Path, "/containers/"):

						removeCalls++

						if !strings.HasSuffix(r.URL.Path, "/containers/"+containerID) {
							t.Errorf("removal did not use Docker ID: %s", r.URL.Path)
						}

						if r.URL.Query().Get("force") != "1" ||
							r.URL.Query().Get("v") != "1" {
							t.Errorf("wrong removal options: %s", r.URL.RawQuery)
						}

						if tc.removeStatus != 0 {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(tc.removeStatus)
							_, _ = fmt.Fprint(w, `{"message":"remove failed"}`)
							return
						}

						w.WriteHeader(http.StatusNoContent)

					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
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

			if created.ID != "" {
				t.Fatalf("unexpected container ID: %q", created.ID)
			}

			if err == nil || !strings.Contains(err.Error(), "create failed") {
				t.Fatalf("original create error lost: %v", err)
			}

			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("missing expected error %q: %v", tc.wantError, err)
			}

			if tc.removeStatus != 0 &&
				!strings.Contains(err.Error(), "remove failed") {
				t.Fatalf("cleanup failure lost: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()

			if observedName != "rew-replay-exec-"+runID ||
				observedRunID != runID ||
				observedToken != runID {
				t.Fatalf(
					"execution identity mismatch: name=%q run=%q token=%q",
					observedName, observedRunID, observedToken,
				)
			}

			if createCalls != 1 || inspectCalls != 1 {
				t.Fatalf(
					"unexpected calls: create=%d inspect=%d",
					createCalls, inspectCalls,
				)
			}

			expectedRemoves := 0
			if tc.wantRemove {
				expectedRemoves = 1
			}

			if removeCalls != expectedRemoves {
				t.Fatalf(
					"unsafe or missing removal: got %d, want %d",
					removeCalls, expectedRemoves,
				)
			}
		})
	}
}
