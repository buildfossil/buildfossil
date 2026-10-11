package capsule

import "testing"

func TestSelectVerifiedGoGraphModsV3(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "example.com/a", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.0.0"},
	}

	full := []GoModuleV3{
		{Path: "example.com/a", Version: "v1.0.0"},
	}

	historical := []GoModuleIdentityV3{
		{Path: "example.com/c", Version: "v1.0.0"},
		{Path: "example.com/d", Version: "v1.0.0"},
	}

	goSum := []byte(
		"example.com/a v1.0.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			"example.com/b v1.0.0/go.mod h1:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=\n" +
			"example.com/c v1.0.0/go.mod h1:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=\n",
	)

	result, err := SelectVerifiedGoGraphModsV3(
		selected, full, historical, goSum,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 ||
		result[0].Path != "example.com/b" ||
		result[1].Path != "example.com/c" {
		t.Fatalf("unexpected graph mod selection: %+v", result)
	}
}

func TestSelectVerifiedGoGraphModsV3RejectsInvalid(t *testing.T) {
	selected := []GoModuleIdentityV3{
		{Path: "../escape", Version: "v1.0.0"},
	}

	if _, err := SelectVerifiedGoGraphModsV3(
		selected, nil, nil, nil,
	); err == nil {
		t.Fatal("invalid module accepted")
	}

	if _, err := SelectVerifiedGoGraphModsV3(
		nil, nil, nil,
		[]byte("invalid checksum line"),
	); err == nil {
		t.Fatal("invalid go.sum accepted")
	}
}
