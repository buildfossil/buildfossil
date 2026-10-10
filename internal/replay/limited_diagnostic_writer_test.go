package replay

import (
	"strings"
	"testing"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

func TestLimitedDiagnosticWriter(t *testing.T) {
	var w limitedDiagnosticWriter

	chunk := strings.Repeat(
		"x",
		int(capsule.MaxGoDependencyDiagnosticsSize),
	)

	n, err := w.Write([]byte(chunk))
	if err != nil || n != len(chunk) {
		t.Fatalf("first write: n=%d err=%v", n, err)
	}

	if w.exceeded {
		t.Fatal("limit should not be exceeded yet")
	}

	n, err = w.Write([]byte("extra"))
	if err != nil || n != 5 {
		t.Fatalf("second write: n=%d err=%v", n, err)
	}

	if !w.exceeded {
		t.Fatal("expected exceeded=true")
	}

	if int64(w.data.Len()) != capsule.MaxGoDependencyDiagnosticsSize {
		t.Fatalf("buffer exceeded size limit: %d", w.data.Len())
	}
}
