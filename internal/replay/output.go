package replay

import "bytes"

const maxReplayOutput = 1 << 20

type boundedOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxReplayOutput - w.buffer.Len()

	if remaining <= 0 {
		if n > 0 {
			w.truncated = true
		}
		return n, nil
	}

	if n > remaining {
		_, _ = w.buffer.Write(p[:remaining])
		w.truncated = true
	} else {
		_, _ = w.buffer.Write(p)
	}

	return n, nil
}

func (w *boundedOutput) Bytes() []byte {
	return w.buffer.Bytes()
}

func (w *boundedOutput) Truncated() bool {
	return w.truncated
}
