package oget

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func TestAll(t *testing.T) {
	outputPath, partsPath := setupDownloadPath(t)
	server := createTestServer(t)
	defer server.Close()

	fileURL := fmt.Sprintf("%s/target.bin", server.URL)
	fileLength := int64(71680)
	sha512Code := "d286fbb1fab9014fdbc543d09f54cb93da6e0f2c809e62ee0c81d69e4bf58eec44571fae192a8da9bc772ce1340a0d51ad638cdba6118909b555a12b005f2930"

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
			FilePath:  filepath.Join(outputPath, "target.bin"),
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
		savedFilePath := filepath.Join(outputPath, "target-parts.bin")

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

	t.Run("download with progress", func(t *testing.T) {
		task, err := CreateGettingTask(&RemoteFile{
			URL: fileURL,
		})
		if err != nil {
			t.Fatalf("create task fail: %s", err)
		}
		var mux sync.Mutex
		savedFilePath := filepath.Join(outputPath, "target-parts.bin")
		events := []ProgressEvent{}

		_, err = task.Get(&GettingConfig{
			FilePath:  savedFilePath,
			PartsPath: partsPath,
			Parts:     4,
			SHA512:    sha512Code,
			ListenProgress: func(event ProgressEvent) {
				mux.Lock()
				events = append(events, event)
				mux.Unlock()
			},
		})
		if err != nil {
			t.Fatalf("download file: %s", err)
		}
		var lastEvent *ProgressEvent = nil
		var phaseCount int = 0

		for _, event := range events {
			if lastEvent != nil {
				if event.Phase < lastEvent.Phase {
					t.Fatalf("unexpected phase: %d", event.Phase)
				}
				if event.Progress < lastEvent.Progress {
					t.Fatalf("unexpected progress: %d", event.Progress)
				}
				if event.Phase > lastEvent.Phase {
					phaseCount = 0
				}
			}
			lastEvent = &event
			phaseCount += 1
		}
		if lastEvent == nil {
			t.Fatalf("no progress event")
		}
		if lastEvent.Phase != Done {
			t.Fatalf("unexpected phase: %d", lastEvent.Phase)
		}
		if phaseCount != 1 {
			t.Fatalf("unexpected phase count: %d", phaseCount)
		}
	})

	t.Run("check response content length and retry utils success", func(t *testing.T) {
		tryDownload := func(mustFail bool) error {
			url := fileURL
			if mustFail {
				url = fmt.Sprintf("%s/target_fail.bin", server.URL)
			}
			task, err := CreateGettingTask(&RemoteFile{
				URL: url,
			})
			if err != nil {
				t.Fatalf("create task fail: %s", err)
			}
			savedFilePath := filepath.Join(outputPath, "target-retry.bin")

			_, err = task.Get(&GettingConfig{
				FilePath:  savedFilePath,
				PartsPath: partsPath,
				Parts:     3,
				SHA512:    sha512Code,
			})
			return err
		}
		err := tryDownload(true)

		if fmt.Sprintf("%s", err) != "download bytes is less than expected" {
			t.Fatalf("unexpected error: %s", err)
		}
		err = tryDownload(false)

		if err != nil {
			t.Fatalf("download file: %s", err)
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

func createTestServer(t *testing.T) *httptest.Server {
	targetPath, err := filepath.Abs("./tests/target.bin")

	if err != nil {
		t.Errorf("Error getting absolute path for %s", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/target.bin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, targetPath)
	})
	mux.HandleFunc("/target_fail.bin", func(w http.ResponseWriter, r *http.Request) {
		file, err := os.Open(targetPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error opening file: %s", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		fileInfo, err := file.Stat()

		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting file info: %s", err), http.StatusInternalServerError)
			return
		}
		maxDownloadSize := fileInfo.Size() / 4

		w.Header().Set("Content-Disposition", "attachment; filename=target_fail.bin")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Accept-Ranges", "bytes")

		rangeHeader := r.Header.Get("Range")

		if rangeHeader != "" {
			ranges := strings.Split(rangeHeader, "=")
			offset := strings.Split(ranges[1], "-")
			startByte, _ := strconv.Atoi(offset[0])
			endByte, _ := strconv.Atoi(offset[1])

			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", startByte, endByte, fileInfo.Size()))

			rangeLength := int64(endByte - startByte + 1)
			copySize := rangeLength

			if rangeLength > maxDownloadSize {
				copySize = maxDownloadSize
			}
			_, _ = file.Seek(int64(startByte), io.SeekStart)
			_, _ = io.CopyN(w, file, copySize)
		} else {
			http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
		}
	})
	return httptest.NewServer(mux)
}
