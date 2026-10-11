package capsule

import (
	"bytes"
	"testing"
)

func TestVerifyGoModuleArtifactSetV3Empty(t *testing.T) {
	if err := VerifyGoModuleArtifactSetV3(
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("empty module set rejected: %v", err)
	}
}

func TestVerifyGoModuleArtifactSetV3RejectsUnexpectedArtifacts(t *testing.T) {
	artifacts := map[string][]byte{
		"module-0001.zip": []byte("unexpected"),
	}

	if err := VerifyGoModuleArtifactSetV3(
		nil,
		artifacts,
		nil,
	); err == nil {
		t.Fatal("unexpected artifacts accepted")
	}
}

func TestVerifyGoModuleArtifactSetV3RejectsMissingArtifacts(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(2, "example.com/b"),
	}

	if err := VerifyGoModuleArtifactSetV3(
		modules,
		nil,
		nil,
	); err == nil {
		t.Fatal("missing module artifacts accepted")
	}
}

func TestVerifyGoModuleArtifactSetV3RejectsWrongIndex(t *testing.T) {
	modules := []GoModuleV3{
		multiModuleFixture(1, "example.com/a"),
		multiModuleFixture(1, "example.com/b"),
	}

	if err := VerifyGoModuleArtifactSetV3(
		modules,
		nil,
		nil,
	); err == nil {
		t.Fatal("incorrect module index accepted")
	}
}

func TestVerifyGoModuleArtifactSetV3TwoRealModules(t *testing.T) {
	first, firstArtifacts, firstSum := makeIndexedGoModuleFixture(
		t, 1, "example.com/a",
	)
	second, secondArtifacts, secondSum := makeIndexedGoModuleFixture(
		t, 2, "example.com/b",
	)

	modules := []GoModuleV3{first, second}
	artifacts := make(map[string][]byte)

	for name, data := range firstArtifacts {
		artifacts[name] = data
	}
	for name, data := range secondArtifacts {
		artifacts[name] = data
	}

	goSum := append(append([]byte(nil), firstSum...), secondSum...)

	if err := VerifyGoModuleArtifactSetV3(
		modules, artifacts, goSum,
	); err != nil {
		t.Fatalf("two valid Go modules rejected: %v", err)
	}

	t.Run("tampered second ZIP", func(t *testing.T) {
		corrupted := make(map[string][]byte, len(artifacts))
		for name, data := range artifacts {
			corrupted[name] = append([]byte(nil), data...)
		}

		corrupted["module-0002.zip"][0] ^= 0xff

		if err := VerifyGoModuleArtifactSetV3(
			modules, corrupted, goSum,
		); err == nil {
			t.Fatal("tampered second module ZIP accepted")
		}
	})

	t.Run("wrong second checksum", func(t *testing.T) {
		corruptedSum := bytes.Replace(
			goSum,
			[]byte("example.com/b v1.0.0 h1:"),
			[]byte("example.com/b v1.0.0 h1:invalid"),
			1,
		)

		if err := VerifyGoModuleArtifactSetV3(
			modules, artifacts, corruptedSum,
		); err == nil {
			t.Fatal("incorrect second module checksum accepted")
		}
	})

	t.Run("extra artifact", func(t *testing.T) {
		extra := make(map[string][]byte, len(artifacts)+1)
		for name, data := range artifacts {
			extra[name] = data
		}
		extra["module-0003.zip"] = []byte("unexpected")

		if err := VerifyGoModuleArtifactSetV3(
			modules, extra, goSum,
		); err == nil {
			t.Fatal("undeclared artifact accepted")
		}
	})
}
