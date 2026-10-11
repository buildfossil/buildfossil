package capsule

import "testing"

func TestVerifyGoGraphInfoArtifactSetV3(t *testing.T) {
	file, _, data := validGraphInfoFixtureV3()

	t.Run("valid", func(t *testing.T) {
		err := VerifyGoGraphInfoArtifactSetV3(
			[]GoGraphInfoFileV3{file},
			map[string][]byte{file.Artifact.Path: data},
		)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		err := VerifyGoGraphInfoArtifactSetV3(
			[]GoGraphInfoFileV3{file},
			nil,
		)
		if err == nil {
			t.Fatal("missing artifact accepted")
		}
	})

	t.Run("extra", func(t *testing.T) {
		err := VerifyGoGraphInfoArtifactSetV3(
			[]GoGraphInfoFileV3{file},
			map[string][]byte{
				file.Artifact.Path: data,
				"unexpected.info":  data,
			},
		)
		if err == nil {
			t.Fatal("extra artifact accepted")
		}
	})

	t.Run("tampered", func(t *testing.T) {
		err := VerifyGoGraphInfoArtifactSetV3(
			[]GoGraphInfoFileV3{file},
			map[string][]byte{
				file.Artifact.Path: []byte(`{"Version":"v1.9.0"}`),
			},
		)
		if err == nil {
			t.Fatal("tampered artifact accepted")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		err := VerifyGoGraphInfoArtifactSetV3(
			[]GoGraphInfoFileV3{file, file},
			map[string][]byte{file.Artifact.Path: data},
		)
		if err == nil {
			t.Fatal("duplicate artifact accepted")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if err := VerifyGoGraphInfoArtifactSetV3(nil, nil); err != nil {
			t.Fatal(err)
		}
	})
}
