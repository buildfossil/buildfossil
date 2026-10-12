package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotExecutionCreateErrorPropagation(t *testing.T) {
	tests := []struct {
		name          string
		inspectStatus int
		foreignToken  bool
		wantError     string
	}{
		{
			name:          "container not found",
			inspectStatus: http.StatusNotFound,
			wantError:     "create failed",
		},
		{
			name:          "inspect failed",
			inspectStatus: http.StatusInternalServerError,
			wantError:     "inspect uncertain snapshot execution container",
		},
		{
			name:          "foreign ownership",
			inspectStatus: http.StatusOK,
			foreignToken:  true,
			wantError:     "unverified ownership",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var createCalls, inspectCalls, removeCalls int
			var name string

			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodPost &&
						strings.HasSuffix(r.URL.Path, "/containers/create"):

						createCalls++
						name = r.URL.Query().Get("name")

						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = fmt.Fprint(w, `{"message":"create failed"}`)

					case r.Method == http.MethodGet &&
						strings.Contains(r.URL.Path, "/containers/"):

						inspectCalls++

						if name == "" ||
							!strings.HasSuffix(r.URL.Path, "/containers/"+name+"/json") {
							t.Errorf("unexpected inspect target: %s", r.URL.Path)
							w.WriteHeader(http.StatusNotFound)
							return
						}

						if tc.inspectStatus != http.StatusOK {
							w.WriteHeader(tc.inspectStatus)
							_, _ = fmt.Fprint(w, `{"message":"inspect failed"}`)
							return
						}

						token := "foreign-token"

						if !tc.foreignToken {
							t.Error("unexpected test configuration")
						}

						w.Header().Set("Content-Type", "application/json")
						_, _ = fmt.Fprintf(
							w,
							`{"Id":"foreign-container-id","Name":%q,"Config":{"Labels":{"org.buildfossil.component":"replay","org.buildfossil.ownership-token":%q}}}`,
							"/"+name,
							token,
						)

					case r.Method == http.MethodDelete &&
						strings.Contains(r.URL.Path, "/containers/"):

						removeCalls++
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

			if created.ID != "" {
				t.Fatalf("unexpected container ID: %s", created.ID)
			}

			if err == nil {
				t.Fatal("expected create error")
			}

			if !strings.Contains(err.Error(), "create failed") {
				t.Fatalf("original create error lost: %v", err)
			}

			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("missing expected error %q: %v", tc.wantError, err)
			}

			if createCalls != 1 || inspectCalls != 1 {
				t.Fatalf(
					"unexpected calls: create=%d inspect=%d",
					createCalls,
					inspectCalls,
				)
			}

			if removeCalls != 0 {
				t.Fatalf("foreign or unverified container removal attempted: %d", removeCalls)
			}
		})
	}
}

func TestSnapshotExecutionCreateAndRemoveErrors(t *testing.T) {
	const containerID = "owned-execution-container-id"

	var containerName string
	var ownershipToken string
	var createCalls, inspectCalls, removeCalls int

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost &&
				strings.HasSuffix(r.URL.Path, "/containers/create"):

				createCalls++
				containerName = r.URL.Query().Get("name")

				var request struct {
					Labels map[string]string `json:"Labels"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode create request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				ownershipToken = request.Labels["org.buildfossil.ownership-token"]

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"message":"injected create failure"}`)

			case r.Method == http.MethodGet &&
				strings.Contains(r.URL.Path, "/containers/"):

				inspectCalls++
				if containerName == "" || ownershipToken == "" ||
					!strings.HasSuffix(
						r.URL.Path,
						"/containers/"+containerName+"/json",
					) {
					t.Errorf("unexpected inspect target or missing identity")
					w.WriteHeader(http.StatusNotFound)
					return
				}

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

				removeCalls++

				if !strings.HasSuffix(
					r.URL.Path,
					"/containers/"+containerID,
				) {
					t.Errorf("container removal not targeted by ID: %s", r.URL.Path)
				}
				if r.URL.Query().Get("force") != "1" ||
					r.URL.Query().Get("v") != "1" {
					t.Errorf("unexpected removal options: %s", r.URL.RawQuery)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"message":"injected remove failure"}`)

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

	if created.ID != "" {
		t.Fatalf("unexpected container ID: %q", created.ID)
	}
	if err == nil {
		t.Fatal("expected combined create and remove errors")
	}
	if !strings.Contains(err.Error(), "injected create failure") {
		t.Fatalf("original create error missing: %v", err)
	}
	if !strings.Contains(err.Error(), "injected remove failure") {
		t.Fatalf("cleanup removal error missing: %v", err)
	}
	if !strings.Contains(
		err.Error(),
		"remove uncertain snapshot execution container",
	) {
		t.Fatalf("cleanup context missing: %v", err)
	}

	if createCalls != 1 || inspectCalls != 1 || removeCalls != 1 {
		t.Fatalf(
			"unexpected Docker API calls: create=%d inspect=%d remove=%d",
			createCalls, inspectCalls, removeCalls,
		)
	}
	if containerName == "" || ownershipToken == "" {
		t.Fatal("missing generated container identity")
	}
}
