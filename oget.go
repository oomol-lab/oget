package oget

import (
	"context"
	"mime"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

type GettingTask struct {
	filename      string
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
		req.Header.Set("Referer", "https://www.referersite.com")
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
		filename:      filename,
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

func (t *GettingTask) Get(config *GettingConfig) {
	// c := config.standardize()

}
