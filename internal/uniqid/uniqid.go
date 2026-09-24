package uniqid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func Suffix() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate random suffix: %w", err)
	}

	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(bytes[:])), nil
}
