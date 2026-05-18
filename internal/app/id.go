package app

import (
	"crypto/rand"
	"encoding/hex"
)

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b[:])
}
