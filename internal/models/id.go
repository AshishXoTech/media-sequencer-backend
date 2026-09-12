package models

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID generates a random 128-bit hex-encoded identifier.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random id: " + err.Error())
	}
	return hex.EncodeToString(b)
}