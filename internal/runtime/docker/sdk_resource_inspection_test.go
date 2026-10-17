package docker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func TestSnapshotVolumeCandidateValidation(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"

	valid := map[string]string{
		"org.rew.component":       "snapshot",
		"org.rew.kind":            "workspace",
		"org.rew.ownership-token": id,
	}

	tests := []struct {
		name   string
		volume string
		labels map[string]string
		want   bool
	}{
		{"valid workspace", "rew-snapshot-" + id, valid, true},
		{"invalid name", "other-" + id, valid, false},
		{"short ID", "rew-snapshot-123", valid, false},
		{"uppercase ID", "rew-snapshot-ABCDEF0123456789abcdef0123456789", valid, false},
		{"missing labels", "rew-snapshot-" + id, nil, false},
		{"wrong component", "rew-snapshot-" + id, map[string]string{
			"org.rew.component":       "other",
			"org.rew.kind":            "workspace",
			"org.rew.ownership-token": id,
		}, false},
		{"wrong kind", "rew-snapshot-" + id, map[string]string{
			"org.rew.component":       "snapshot",
			"org.rew.kind":            "unknown",
			"org.rew.ownership-token": id,
		}, false},
		{"missing token", "rew-snapshot-" + id, map[string]string{
			"org.rew.component": "snapshot",
			"org.rew.kind":      "workspace",
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := isSnapshotVolumeCandidate(tt.volume, tt.labels)
			if got != tt.want {
				t.Fatalf("candidate = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListSnapshotVolumeCandidatesReadOnly(t *testing.T) {
	const id1 = "0123456789abcdef0123456789abcdef"
	const id2 = "fedcba9876543210fedcba9876543210"

	var requestCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.Method != http.MethodGet ||
			!strings.HasSuffix(r.URL.Path, "/volumes") {
			t.Errorf("unexpected Docker API request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"Volumes":[
			{"Name":"rew-snapshot-%s","Driver":"local","Labels":{
				"org.rew.component":"snapshot",
				"org.rew.kind":"module-cache",
				"org.rew.ownership-token":"%s"
			}},
			{"Name":"foreign-volume","Driver":"local","Labels":{
				"org.rew.component":"snapshot",
				"org.rew.kind":"workspace",
				"org.rew.ownership-token":"%s"
			}},
			{"Name":"rew-snapshot-%s","Driver":"local","Labels":{
				"org.rew.component":"snapshot",
				"org.rew.kind":"workspace",
				"org.rew.ownership-token":"%s"
			}}
		]}`, id2, id2, id1, id1, id1)
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

	got, err := ListSnapshotVolumeCandidates(context.Background(), dockerClient)
	if err != nil {
		t.Fatal(err)
	}

	if requestCount != 1 {
		t.Fatalf("Docker API requests = %d, want 1", requestCount)
	}

	if len(got) != 2 {
		t.Fatalf("candidates = %+v, want 2", got)
	}

	if got[0].Name != "rew-snapshot-"+id1 ||
		got[0].Kind != "workspace" ||
		got[1].Name != "rew-snapshot-"+id2 ||
		got[1].Kind != "module-cache" {
		t.Fatalf("unexpected candidates or sorting: %+v", got)
	}
}

func TestListSnapshotVolumeCandidatesErrors(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		_, err := ListSnapshotVolumeCandidates(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "nil client") {
			t.Fatalf("expected nil client error, got %v", err)
		}
	})

	t.Run("Docker API failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("unexpected mutation request: %s", r.Method)
			}
			http.Error(w, `{"message":"daemon unavailable"}`, http.StatusInternalServerError)
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

		_, err = ListSnapshotVolumeCandidates(context.Background(), dockerClient)
		if err == nil {
			t.Fatal("expected Docker API error")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request after cancellation: %s", r.URL.Path)
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

		_, err = ListSnapshotVolumeCandidates(ctx, dockerClient)
		if err == nil {
			t.Fatal("expected context cancellation error")
		}
	})
}

func TestListSnapshotVolumeCandidatesWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet ||
			!strings.HasSuffix(r.URL.Path, "/volumes") {
			t.Errorf("unexpected Docker API request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"Volumes": [],
			"Warnings": ["volume listing incomplete"]
		}`)
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

	candidates, err := ListSnapshotVolumeCandidates(
		context.Background(),
		dockerClient,
	)
	if err == nil || !strings.Contains(err.Error(), "incomplete volume inspection") {
		t.Fatalf("expected incomplete inspection error, got %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("unexpected candidates on incomplete inspection: %+v", candidates)
	}
}
