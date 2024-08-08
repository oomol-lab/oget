package oget

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func TestAll(t *testing.T) {

	fileURL := "https://raw.githubusercontent.com/oomol-lab/ovm-js/d90e10fddc3750fc69c3e00128bcfa03823f7af7/README.md"
	fileLength := int64(5003)
	sha512Code := "562f213fc1bd2ad1bc70bb54a332b3aeaf5338b85d4bbe6b512fa4fc0e379c655054081292d38ea8bf3e5ec0cecfe263c3918567caa5937aa0035d3c40a8feb8"
	outputPath, partsPath := setupDownloadPath(t)

	t.Run("file info", func(t *testing.T) {
		task, err := CreateGettingTask(&RemoteFile{
			URL: fileURL,
		})
		if err != nil {
			t.Fatalf("create task fail: %s", err)
		}
		if task.ContentLength() != fileLength {
			t.Fatalf("unexpected content length: %d", task.ContentLength())
		}
	})

	t.Run("download without parts", func(t *testing.T) {
		task, err := CreateGettingTask(&RemoteFile{
			URL: fileURL,
		})
		if err != nil {
			t.Fatalf("create task fail: %s", err)
		}
		_, err = task.Get(&GettingConfig{
			FilePath:  filepath.Join(outputPath, "README.md"),
			PartsPath: partsPath,
			SHA512:    sha512Code,
		})
		if err != nil {
			t.Fatalf("download file: %s", err)
		}
	})

	t.Run("download with parts", func(t *testing.T) {
		task, err := CreateGettingTask(&RemoteFile{
			URL: fileURL,
		})
		if err != nil {
			t.Fatalf("create task fail: %s", err)
		}
		savedFilePath := filepath.Join(outputPath, "NEXT-README.md")

		_, err = task.Get(&GettingConfig{
			FilePath:  savedFilePath,
			PartsPath: partsPath,
			Parts:     4,
			SHA512:    sha512Code,
		})
		if err != nil {
			t.Fatalf("download file: %s", err)
		}
		savedFileCode, err := SHA512(savedFilePath)

		if err != nil {
			t.Fatalf("get code of sha512 fail: %s", err)
		}
		if sha512Code != savedFileCode {
			t.Fatalf("unexpected sha512 code: %s", savedFileCode)
		}
	})
}

func setupDownloadPath(t *testing.T) (string, string) {
	downloadingPath, err := filepath.Abs("./downloading")

	if err != nil {
		t.Errorf("Error getting absolute path for %s", err)
	}
	if _, err := os.Stat(downloadingPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(downloadingPath); err != nil {
			t.Fatalf("Error deleting directory: %s", err)
		}
	}
	outputPath := filepath.Join(downloadingPath, "output")
	partsPath := filepath.Join(downloadingPath, "parts")

	err = os.MkdirAll(outputPath, os.ModePerm)

	if err != nil {
		t.Fatalf("Error creating directory: %s", err)
	}
	err = os.MkdirAll(partsPath, os.ModePerm)

	if err != nil {
		t.Fatalf("Error creating directory: %s", err)
	}
	return outputPath, partsPath
}
