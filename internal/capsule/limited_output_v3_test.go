package capsule

import "testing"

func TestLimitedOutputV3(t *testing.T) {
	var output = limitedOutputV3{limit: 5}

	if n, err := output.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("initial write failed: n=%d err=%v", n, err)
	}

	if n, err := output.Write([]byte("de")); err != nil || n != 2 {
		t.Fatalf("write up to limit failed: n=%d err=%v", n, err)
	}

	if output.String() != "abcde" {
		t.Fatalf("unexpected output: %q", output.String())
	}

	if n, err := output.Write([]byte("f")); err == nil || n != 0 {
		t.Fatalf("expected overflow error: n=%d err=%v", n, err)
	}

	if output.String() != "abcde" {
		t.Fatal("buffer changed after rejected write")
	}
}
