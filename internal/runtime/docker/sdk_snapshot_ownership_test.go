package docker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func TestCleanupUnknownPreparationContainerOwnership(t *testing.T) {
	tests := []struct {
		name           string
		inspectStatus  int
		inspectToken   string
		inspectName    string
		component      string
		removeStatus   int
		wantErr        string
		wantRemoveCall bool
	}{
		{
			name:          "foreign ownership",
			inspectStatus: http.StatusOK,
			inspectToken:  "foreign-token",
			wantErr:       "unverified ownership",
		},
		{
			name:          "wrong container name",
			inspectStatus: http.StatusOK,
			inspectToken:  "expected-token",
			inspectName:   "/foreign-container",
			wantErr:       "unverified ownership",
		},
		{
			name:          "wrong component label",
			inspectStatus: http.StatusOK,
			inspectToken:  "expected-token",
			component:     "foreign",
			wantErr:       "unverified ownership",
		},
		{
			name:          "container not found",
			inspectStatus: http.StatusNotFound,
		},
		{
			name:          "inspect failure",
			inspectStatus: http.StatusInternalServerError,
			wantErr:       "inspect uncertain preparation container",
		},
		{
			name:           "remove failure",
			inspectStatus:  http.StatusOK,
			inspectToken:   "expected-token",
			removeStatus:   http.StatusInternalServerError,
			wantErr:        "remove uncertain preparation container",
			wantRemoveCall: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const containerName = "buildfossil-snapshot-prep-test"
			const containerID = "mock-owned-container-id"
			const expectedToken = "expected-token"

			var inspectCalls, removeCalls int
			var removedByID bool

			inspectName := tc.inspectName
			if inspectName == "" {
				inspectName = "/" + containerName
			}

			component := tc.component
			if component == "" {
				component = "replay"
			}

			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodGet &&
						strings.HasSuffix(r.URL.Path, "/containers/"+containerName+"/json"):

						inspectCalls++

						if tc.inspectStatus != http.StatusOK {
							w.WriteHeader(tc.inspectStatus)
							_, _ = fmt.Fprint(w, `{"message":"mock inspect error"}`)
							return
						}

						w.Header().Set("Content-Type", "application/json")
						_, _ = fmt.Fprintf(
							w,
							`{"Id":%q,"Name":%q,"Config":{"Labels":{"org.buildfossil.component":%q,"org.buildfossil.ownership-token":%q}}}`,
							containerID,
							inspectName,
							component,
							tc.inspectToken,
						)

					case r.Method == http.MethodDelete &&
						strings.Contains(r.URL.Path, "/containers/"):

						removeCalls++
						removedByID = strings.Contains(
							r.URL.Path,
							"/containers/"+containerID,
						)

						if tc.removeStatus != 0 &&
							tc.removeStatus != http.StatusNoContent {
							w.WriteHeader(tc.removeStatus)
							_, _ = fmt.Fprint(w, `{"message":"mock remove error"}`)
							return
						}

						w.WriteHeader(http.StatusNoContent)

					default:
						t.Errorf("unexpected Docker API request: %s %s", r.Method, r.URL.Path)
						w.WriteHeader(http.StatusInternalServerError)
					}
				},
			))
			defer server.Close()

			dockerClient, err := client.New(
				client.WithHost(
					"tcp://"+strings.TrimPrefix(server.URL, "http://"),
				),
				client.WithHTTPClient(server.Client()),
				client.WithAPIVersion("1.44"),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer dockerClient.Close()

			err = cleanupUnknownPreparationContainer(
				dockerClient,
				containerName,
				expectedToken,
			)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected cleanup error: %v", err)
				}
			} else if err == nil ||
				!strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("cleanup error = %v, want %q", err, tc.wantErr)
			}

			if inspectCalls != 1 {
				t.Fatalf("inspect calls = %d, want 1", inspectCalls)
			}

			if tc.wantRemoveCall {
				if removeCalls != 1 || !removedByID {
					t.Fatalf(
						"expected removal by ID: calls=%d byID=%t",
						removeCalls,
						removedByID,
					)
				}
			} else if removeCalls != 0 {
				t.Fatalf(
					"unsafe removal attempted: %d calls",
					removeCalls,
				)
			}
		})
	}
}
