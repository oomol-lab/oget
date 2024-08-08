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
	var prog *progress
	c := config.standardize()
	tasks := []*subTask{}

	if c.ListenProgress != nil {
		prog = downloadingProgress(t.contentLength, c.ListenProgress)
	}
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
			if len(tasks) > 1 || !tasks[0].overrideFile {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.begin, task.end))
			}
			if err := t.downloadToFile(req, task, prog); err != nil {
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return clean, err
	}
	if prog != nil {
		prog = prog.toCopingPhase()
	}
	if err := t.mergeFile(&c, prog); err != nil {
		return clean, err
	}
	if prog != nil {
		prog.fireDone()
	}
	clean = func() error { return nil }

	return clean, nil
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

func (t *GettingTask) downloadToFile(req *http.Request, task *subTask, prog *progress) error {
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

	var respReader io.Reader = resp.Body
	if prog != nil {
		respReader = prog.reader(respReader)
	}
	written, err := io.Copy(output, respReader)

	if err != nil {
		return errors.Wrapf(err, "failed to write response body")
	}
	wantSize := task.end - task.begin + 1
	if written < wantSize {
		return errors.New("download bytes is less than expected")
	}
	return nil
}

func (t *GettingTask) mergeFile(c *GettingConfig, prog *progress) error {
	err := os.MkdirAll(c.dirPath(), 0755)
	if err != nil {
		return errors.Wrapf(err, "make directory failed")
	}
	partPathList := []string{}
	for i := 0; i < c.Parts; i++ {
		partPath := filepath.Join(c.PartsPath, c.partFileName(i))
		partPathList = append(partPathList, partPath)
	}
	if c.SHA512 != "" {
		code, err := sha512OfFiles(&partPathList)
		if err != nil {
			return errors.Wrapf(err, "failed to get sha512 code")
		}
		if code != c.SHA512 {
			return createSHA512Error("sha512 code does not match")
		}
	}
	if len(partPathList) == 1 {
		partPath := partPathList[0]
		err := os.Rename(partPath, c.FilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to move file")
		}
	} else {
		targetFile, err := os.Create(c.FilePath)
		if err != nil {
			return errors.Wrap(err, "failed to create a file in download location")
		}
		defer targetFile.Close()

		for _, partPath := range partPathList {
			subFile, err := os.Open(partPath)
			if err != nil {
				return errors.Wrapf(err, "failed to open file in download location")
			}
			defer subFile.Close()

			var reader io.Reader = subFile
			if prog != nil {
				reader = prog.reader(reader)
			}
			_, err = io.Copy(targetFile, reader)
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
		partPath := filepath.Join(c.PartsPath, c.partFileName(i))
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
	end := int64(0)

	if index == c.Parts-1 {
		end = t.contentLength - 1
	} else {
		end = begin + chunkSize - 1
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
