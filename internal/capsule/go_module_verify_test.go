package capsule

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func makeGoModuleFixture(t *testing.T) (
	GoModuleV3,
	map[string][]byte,
	[]byte,
) {
	t.Helper()

	const modulePath = "example.com/buildfossil/fixture"
	const moduleVersion = "v1.0.0"

	modData := []byte("module " + modulePath + "\n\ngo 1.26.0\n")
	sourceData := []byte(
		"package fixture\n\nfunc Value() string { return \"offline-ok\" }\n",
	)

	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{modulePath + "@" + moduleVersion + "/go.mod", modData},
		{modulePath + "@" + moduleVersion + "/fixture.go", sourceData},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	artifacts := map[string][]byte{
		"module-0001.zip":  append([]byte(nil), buffer.Bytes()...),
		"module-0001.mod":  modData,
		"module-0001.info": []byte(`{"Version":"v1.0.0"}`),
	}

	zipPath := filepath.Join(t.TempDir(), "module.zip")
	if err := os.WriteFile(zipPath, artifacts["module-0001.zip"], 0600); err != nil {
		t.Fatal(err)
	}

	zipHash, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

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

	goSum := []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		modulePath, moduleVersion, zipHash,
		modulePath, moduleVersion, modHash,
	))

	mod := GoModuleV3{
		Path:    modulePath,
		Version: moduleVersion,
	}

	for _, name := range []string{
		"module-0001.zip",
		"module-0001.mod",
		"module-0001.info",
	} {
		data := artifacts[name]
		mod.Artifacts = append(mod.Artifacts, GoModuleArtifact{
			Path:   name,
			Size:   int64(len(data)),
			SHA256: SHA256(data),
		})
	}

	return mod, artifacts, goSum
}

func TestVerifyGoModuleArtifactsV3Valid(t *testing.T) {
	mod, artifacts, goSum := makeGoModuleFixture(t)

	if err := VerifyGoModuleArtifactsV3(mod, artifacts, goSum); err != nil {
		t.Fatalf("valid Go module rejected: %v", err)
	}
}

func TestVerifyGoModuleArtifactsV3RejectsInvalidArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GoModuleV3, map[string][]byte, *[]byte)
	}{
		{
			name: "modified ZIP",
			mutate: func(_ *GoModuleV3, artifacts map[string][]byte, _ *[]byte) {
				artifacts["module-0001.zip"][0] ^= 0xff
			},
		},
		{
			name: "invalid ZIP with matching SHA256",
			mutate: func(mod *GoModuleV3, artifacts map[string][]byte, _ *[]byte) {
				data := []byte("not a ZIP archive")
				artifacts["module-0001.zip"] = data
				mod.Artifacts[0].Size = int64(len(data))
				mod.Artifacts[0].SHA256 = SHA256(data)
			},
		},
		{
			name: "wrong Go checksum",
			mutate: func(_ *GoModuleV3, _ map[string][]byte, goSum *[]byte) {
				*goSum = bytes.Replace(
					*goSum,
					[]byte("h1:"),
					[]byte("h1:incorrect-"),
					1,
				)
			},
		},
		{
			name: "missing Go checksum",
			mutate: func(_ *GoModuleV3, _ map[string][]byte, goSum *[]byte) {
				lines := bytes.Split(*goSum, []byte("\n"))
				*goSum = append([]byte(nil), lines[1]...)
			},
		},
		{
			name: "module path mismatch",
			mutate: func(mod *GoModuleV3, artifacts map[string][]byte, _ *[]byte) {
				data := []byte("module example.com/another/module\n\ngo 1.26.0\n")
				artifacts["module-0001.mod"] = data
				mod.Artifacts[1].Size = int64(len(data))
				mod.Artifacts[1].SHA256 = SHA256(data)
			},
		},
		{
			name: "module version mismatch",
			mutate: func(mod *GoModuleV3, artifacts map[string][]byte, _ *[]byte) {
				data := []byte(`{"Version":"v2.0.0"}`)
				artifacts["module-0001.info"] = data
				mod.Artifacts[2].Size = int64(len(data))
				mod.Artifacts[2].SHA256 = SHA256(data)
			},
		},
		{
			name: "missing artifact",
			mutate: func(_ *GoModuleV3, artifacts map[string][]byte, _ *[]byte) {
				delete(artifacts, "module-0001.info")
			},
		},
		{
			name: "duplicate go.sum entry",
			mutate: func(_ *GoModuleV3, _ map[string][]byte, goSum *[]byte) {
				lines := bytes.SplitN(*goSum, []byte("\n"), 2)
				*goSum = append(*goSum, lines[0]...)
				*goSum = append(*goSum, '\n')
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, artifacts, goSum := makeGoModuleFixture(t)
			tt.mutate(&mod, artifacts, &goSum)

			if err := VerifyGoModuleArtifactsV3(mod, artifacts, goSum); err == nil {
				t.Fatal("invalid Go module artifacts accepted")
			}
		})
	}
}

func TestVerifyGoModuleZipContentsRejectsOversizedUnpackedData(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "large.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	w, err := zw.Create(
		"example.com/buildfossil/fixture@v1.0.0/large.txt",
	)
	if err != nil {
		t.Fatal(err)
	}

	data := bytes.Repeat(
		[]byte("A"),
		int(MaxGoModuleUnpackedSizeV3+1),
	)

	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err == nil {
		t.Fatal("expected unpacked size limit error")
	}
}

func TestVerifyGoModuleZipContentsRejectsSymlink(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "symlink.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	header := &zip.FileHeader{
		Name: "example.com/buildfossil/fixture@v1.0.0/link",
	}
	header.SetMode(os.ModeSymlink | 0777)

	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Write([]byte("../../etc/passwd")); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestVerifyGoModuleZipContentsRejectsCorruptedEntry(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "corrupt.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	header := &zip.FileHeader{
		Name:   "example.com/buildfossil/fixture@v1.0.0/data.txt",
		Method: zip.Store,
	}

	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Write([]byte("original-content")); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	index := bytes.Index(data, []byte("original-content"))
	if index < 0 {
		t.Fatal("could not locate ZIP entry data")
	}

	data[index] ^= 0xff

	if err := os.WriteFile(zipPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err == nil {
		t.Fatal("expected corrupted ZIP entry rejection")
	}
}

func TestVerifyGoModuleZipContentsAcceptsValidArchive(t *testing.T) {
	mod, artifacts, _ := makeGoModuleFixture(t)

	zipPath := filepath.Join(t.TempDir(), "valid.zip")

	if err := os.WriteFile(
		zipPath,
		artifacts["module-0001.zip"],
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err != nil {
		t.Fatalf(
			"valid ZIP rejected for %s@%s: %v",
			mod.Path,
			mod.Version,
			err,
		)
	}
}

func TestVerifyGoModuleZipContentsRejectsCumulativeSize(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "cumulative.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	for _, name := range []string{"first.bin", "second.bin"} {
		w, err := zw.Create(
			"example.com/buildfossil/fixture@v1.0.0/" + name,
		)
		if err != nil {
			t.Fatal(err)
		}

		data := bytes.Repeat(
			[]byte("A"),
			int(MaxGoModuleUnpackedSizeV3/2+1),
		)

		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := verifyGoModuleZipContents(zipPath); err == nil {
		t.Fatal("expected cumulative ZIP size rejection")
	}
}
