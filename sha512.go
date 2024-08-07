package oget

import (
	"crypto/sha512"
	"fmt"
	"io"
	"os"
)

func SHA512(path string) (string, error) {
	return sha512OfFiles(&[]string{path})
}

func sha512OfFiles(pathList *[]string) (string, error) {
	h := sha512.New()
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

type SHA512Error struct {
	message string
}

func (e SHA512Error) Error() string {
	return e.message
}

func createSHA512Error(message string) SHA512Error {
	return SHA512Error{message}
}
