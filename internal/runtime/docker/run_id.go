package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newRunID() (string, error) {
	var id [16]byte

	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("docker SDK: generate run ID: %w", err)
	}

	return hex.EncodeToString(id[:]), nil
}
