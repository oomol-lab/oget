package oget

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type GettingTask struct {
	url           string
	filename      string
	useragent     string
	referer       string
	contentLength int64
	client        *http.Client
	context       context.Context
	timeout       time.Duration
}

func CreateGettingTask(config *RemoteFile) (*GettingTask, error) {
	c := config.standardize()
	client := newGettingClient(c.MaxIdleConnsPerHost)
	ctx, cancel := context.WithTimeout(c.Context, c.Timeout)
	defer cancel()

	req, err := http.NewRequest("HEAD", c.URL, nil)

	if err != nil {
		return nil, errors.Wrap(err, "failed to make head request")
	}
	req = req.WithContext(ctx)

	if config.Useragent != "" {
		req.Header.Set("User-Agent", config.Useragent)
	}
	if config.Referer != "" {
		req.Header.Set("Referer", config.Referer)
	}
	resp, err := client.Do(req)

	if err != nil {
		return nil, errors.Wrap(err, "failed to head request")
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, errors.New("does not support range request")
	}
	if resp.ContentLength <= 0 {
		return nil, errors.New("invalid content length")
	}
	filename := ""
	_, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if len(params) > 0 && params["filename"] != "" {
		filename = params["filename"]
	}
	task := &GettingTask{
		url:           c.URL,
		filename:      filename,
		useragent:     c.Useragent,
		referer:       c.Referer,
		contentLength: resp.ContentLength,
		client:        client,
		context:       c.Context,
		timeout:       c.Timeout,
	}
	return task, nil
}

func (t *GettingTask) ContentLength() int64 {
	return t.contentLength
}

func (t *GettingTask) Get(config *GettingConfig) (func() error, error) {
	c := config.standardize()
	tasks := []*subTask{}

	for i := 0; i < c.Parts; i++ {
		task := t.getPartTask(&c, i)
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	clean := func() error {
		return t.cleanPartFiles(&c)
	}
	if len(tasks) > 0 {
		err := os.MkdirAll(c.PartsPath, 0755)
		if err != nil {
			return clean, err
		}
	}
	eg, ctx := errgroup.WithContext(t.context)

	for _, task := range tasks {
		task := task
		eg.Go(func() error {
			req, err := t.createRequest(ctx)
			if err != nil {
				return err
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.begin, task.end))
			if err := t.downloadToFile(req, task); err != nil {
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return clean, err
	}
	if err := t.mergeFile(&c); err != nil {
		return clean, err
	}
	return func() error { return nil }, nil
}

func (t *GettingTask) createRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequest("GET", t.url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make a new request")
	}
	req = req.WithContext(ctx)

	if t.useragent != "" {
		req.Header.Set("User-Agent", t.useragent)
	}
	if t.referer != "" {
		req.Header.Set("Referer", t.referer)
	}
	return req, nil
}

func (t *GettingTask) downloadToFile(req *http.Request, task *subTask) error {
	resp, err := t.client.Do(req)

	if err != nil {
		return errors.Wrapf(err, "failed to get response: %q", err)
	}
	defer resp.Body.Close()

	flag := os.O_WRONLY | os.O_CREATE

	if !task.overrideFile {
		flag |= os.O_APPEND
	}
	output, err := os.OpenFile(task.path, flag, 0666)

	if err != nil {
		return errors.Wrapf(err, "failed to write file")
	}
	defer output.Close()

	if _, err := io.Copy(output, resp.Body); err != nil {
		return errors.Wrapf(err, "failed to write response body")
	}
	return nil
}

func (t *GettingTask) mergeFile(c *GettingConfig) error {
	err := os.MkdirAll(c.dirPath(), 0755)

	if err != nil {
		return err
	}
	if c.Parts == 1 {
		partPath := filepath.Join(c.PartsPath, c.partFileName(0))
		err := os.Rename(partPath, c.FilePath)
		if err != nil {
			return err
		}
	} else {
		targetFile, err := os.Create(c.FilePath)
		if err != nil {
			return errors.Wrap(err, "failed to create a file in download location")
		}
		defer targetFile.Close()

		for i := 0; i < c.Parts; i++ {
			partPath := filepath.Join(c.PartsPath, c.partFileName(i))
			subFile, err := os.Open(partPath)
			if err != nil {
				return errors.Wrapf(err, "failed to open file in download location")
			}
			defer subFile.Close()
			_, err = io.Copy(targetFile, subFile)
			if err != nil {
				errors.Wrapf(err, "failed to copy part of file")
			}
		}
		t.cleanPartFiles(c)
	}
	return nil
}

func (t *GettingTask) cleanPartFiles(c *GettingConfig) error {
	for i := 0; i < c.Parts; i++ {
		partPath := filepath.Join(c.PartsPath, c.partFileName(0))
		if err := os.Remove(partPath); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

type subTask struct {
	begin        int64
	end          int64
	path         string
	overrideFile bool
}

func (t *GettingTask) getPartTask(c *GettingConfig, index int) *subTask {
	chunkSize := t.contentLength / int64(c.Parts)
	begin := chunkSize * int64(index)
	end := begin + chunkSize - 1

	if end >= t.contentLength-1 {
		end = t.contentLength - 1
	}
	filePath := filepath.Join(c.PartsPath, c.partFileName(index))
	info, err := os.Stat(filePath)
	overrideFile := false

	if err != nil {
		overrideFile = true
	} else {
		begin += info.Size()
		if begin > end {
			return nil
		}
	}
	return &subTask{
		begin:        begin,
		end:          end,
		path:         filePath,
		overrideFile: overrideFile,
	}
}
