package execution

import "bytes"

const MaxCapturedOutput = 1 << 20 // 1 MiB

type limitedWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedWriter(limit int) *limitedWriter {
	return &limitedWriter{limit: limit}
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	remaining := w.limit - w.buf.Len()

	if remaining <= 0 {
		if n > 0 {
			w.truncated = true
		}
		return n, nil
	}

	if n > remaining {
		_, _ = w.buf.Write(p[:remaining])
		w.truncated = true
	} else {
		_, _ = w.buf.Write(p)
	}

	return n, nil
}

func (w *limitedWriter) String() string {
	return w.buf.String()
}

func (w *limitedWriter) Truncated() bool {
	return w.truncated
}
