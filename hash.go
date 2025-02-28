package oget

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
)

type HashType string

const (
	SHA256 HashType = "sha256"
	SHA512 HashType = "sha512"
)

func supportHashType(s HashType) bool {
	return s == SHA256 || s == SHA512
}

func SHA(path string, ht HashType) (string, error) {
	return shaOfFiles(&[]string{path}, ht)
}

func shaOfFiles(pathList *[]string, ht HashType) (string, error) {
	var h hash.Hash

	switch ht {
	case SHA256:
		h = sha256.New()
	case SHA512:
		h = sha512.New()
	}

	for _, path := range *pathList {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		io.Copy(h, file)
	}
	hashInBytes := h.Sum(nil)
	hashStr := fmt.Sprintf("%x", hashInBytes)

	return hashStr, nil
}

type HashError struct {
	message string
}

func (e HashError) Error() string {
	return e.message
}

func createHashError(message string) HashError {
	return HashError{message}
}
