package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func generateSHA256Hash(file io.Reader) string {
	hash := sha256.New()

	_, err := io.Copy(hash, file)
	if err != nil {
		logger.Panicln(err)
	}

	return hex.EncodeToString(hash.Sum(nil))
}
