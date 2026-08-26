package workflow

import (
	"crypto/rand"
	"encoding/hex"
)

type RandomIDs struct{}

func (RandomIDs) New() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type IDSource interface{ New() string }
