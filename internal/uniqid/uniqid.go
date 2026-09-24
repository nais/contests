package uniqid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func Suffix() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(bytes[:]))
}
