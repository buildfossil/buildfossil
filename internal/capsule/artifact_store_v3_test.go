package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"
)

func artifactDigestV3(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func TestArtifactStoreV3RoundTripAndCleanup(t *testing.T) {
	store, err := NewArtifactStoreV3(10)
	if err != nil {
		t.Fatal(err)
	}
	dir := store.dir

	if err := store.Put("module-0001.zip", strings.NewReader("hello"),
		5, artifactDigestV3("hello")); err != nil {
		t.Fatal(err)
	}
	if store.Used() != 5 {
		t.Fatalf("used = %d, want 5", store.Used())
	}

	reader, err := store.Open("module-0001.zip")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(got) != "hello" {
		t.Fatalf("got %q, error %v", got, err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary directory still exists: %v", err)
	}
	if _, err := store.Open("module-0001.zip"); err == nil {
		t.Fatal("closed store allowed reading")
	}
}

func TestArtifactStoreV3RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		size   int64
		digest string
	}{
		{"../escape", "ok", 2, artifactDigestV3("ok")},
		{"nested/file", "ok", 2, artifactDigestV3("ok")},
		{"module.zip", "abcdef", 6, artifactDigestV3("abcdef")},
		{"module.zip", "abc", 4, artifactDigestV3("abc")},
		{"module.zip", "abc", 3, artifactDigestV3("wrong")},
		{"module.zip", "abc", 3, "not-a-digest"},
	}

	for i, tc := range cases {
		t.Run(strings.ReplaceAll(tc.name, "/", "_")+
			"-"+string(rune('a'+i)), func(t *testing.T) {
			store, err := NewArtifactStoreV3(5)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()

			err = store.Put(tc.name, strings.NewReader(tc.data),
				tc.size, tc.digest)
			if err == nil {
				t.Fatal("invalid input accepted")
			}
			if store.Used() != 0 {
				t.Fatal("failed write consumed budget")
			}
			if _, err := store.Open(tc.name); err == nil {
				t.Fatal("failed artifact became readable")
			}
		})
	}
}

func TestArtifactStoreV3DuplicateAndBudget(t *testing.T) {
	store, err := NewArtifactStoreV3(5)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Put("a.zip", strings.NewReader("abc"),
		3, artifactDigestV3("abc")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("a.zip", strings.NewReader("abc"),
		3, artifactDigestV3("abc")); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := store.Put("b.zip", strings.NewReader("xyz"),
		3, artifactDigestV3("xyz")); err == nil {
		t.Fatal("budget overflow accepted")
	}
	if err := store.Put("b.zip", strings.NewReader("xy"),
		2, artifactDigestV3("xy")); err != nil {
		t.Fatal(err)
	}
	if store.Used() != 5 {
		t.Fatalf("used = %d, want 5", store.Used())
	}
}
