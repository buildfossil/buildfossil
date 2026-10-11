package capsule

import (
	"bytes"
	"fmt"
)

type limitedOutputV3 struct {
	buffer bytes.Buffer
	limit  int
}

func (w *limitedOutputV3) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buffer.Len() {
		return 0, fmt.Errorf("capsule: Go process output exceeds limit")
	}
	return w.buffer.Write(p)
}

func (w *limitedOutputV3) Bytes() []byte {
	return w.buffer.Bytes()
}

func (w *limitedOutputV3) String() string {
	return w.buffer.String()
}
