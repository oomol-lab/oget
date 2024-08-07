package oget

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestAll(t *testing.T) {

	t.Run("file info", func(t *testing.T) {
		fileURL := "https://raw.githubusercontent.com/oomol-lab/ovm-js/d90e10fddc3750fc69c3e00128bcfa03823f7af7/README.md"
		fileLength := int64(5003)
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
}
