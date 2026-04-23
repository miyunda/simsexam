package bootstrap

import (
	"crypto/sha256"
	"fmt"
)

func sha256sum(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum[:])
}
