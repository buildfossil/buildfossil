package replay

import (
	"bytes"

	"github.com/buildfossil/buildfossil/internal/capsule"
)

type limitedDiagnosticWriter struct {
	data     bytes.Buffer
	exceeded bool
}

func (w *limitedDiagnosticWriter) Write(p []byte) (int, error) {
	limit := capsule.MaxGoDependencyDiagnosticsSize
	remaining := limit - int64(w.data.Len())

	if remaining <= 0 {
		if len(p) > 0 {
			w.exceeded = true
		}
		return len(p), nil
	}

	if int64(len(p)) > remaining {
		w.exceeded = true
		_, _ = w.data.Write(p[:int(remaining)])
		return len(p), nil
	}

	return w.data.Write(p)
}
