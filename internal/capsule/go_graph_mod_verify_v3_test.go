package capsule

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestVerifyGoGraphModFileV3(t *testing.T) {
	const modulePath = "example.com/graph"
	const version = "v1.0.0"

	modData := []byte("module example.com/graph\n\ngo 1.23\n")

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file: %s", name)
			}
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	file := GoGraphModFileV3{
		Path:    modulePath,
		Version: version,
		Artifact: GoModuleArtifact{
			Path:   "graph-mod-0001.mod",
			Size:   int64(len(modData)),
			SHA256: SHA256(modData),
		},
	}

	goSum := []byte(fmt.Sprintf(
		"%s %s/go.mod %s\n",
		modulePath, version, modHash,
	))

	if err := VerifyGoGraphModFileV3(file, modData, goSum); err != nil {
		t.Fatalf("valid graph mod rejected: %v", err)
	}

	t.Run("tampered content", func(t *testing.T) {
		tampered := append([]byte(nil), modData...)
		tampered[0] ^= 0xff

		if err := VerifyGoGraphModFileV3(file, tampered, goSum); err == nil {
			t.Fatal("tampered graph mod accepted")
		}
	})

	t.Run("wrong module identity", func(t *testing.T) {
		wrong := []byte("module example.com/other\n\ngo 1.23\n")
		mutated := file
		mutated.Artifact.SHA256 = SHA256(wrong)
		mutated.Artifact.Size = int64(len(wrong))

		if err := VerifyGoGraphModFileV3(mutated, wrong, goSum); err == nil {
			t.Fatal("wrong module identity accepted")
		}
	})

	t.Run("missing go.sum checksum", func(t *testing.T) {
		if err := VerifyGoGraphModFileV3(file, modData, nil); err == nil {
			t.Fatal("missing checksum accepted")
		}
	})

	t.Run("incorrect go.sum checksum", func(t *testing.T) {
		wrongSum := []byte(fmt.Sprintf(
			"%s %s/go.mod h1:invalid\n",
			modulePath, version,
		))

		if err := VerifyGoGraphModFileV3(file, modData, wrongSum); err == nil {
			t.Fatal("incorrect checksum accepted")
		}
	})
}
