package oget

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

type RemoteFile struct {
	// the context for the http request.
	// if the value is nil, the context will be context.Background().
	Context context.Context
	// the maximum amount of time a dial will wait for a connect or copy to complete.
	// the default is 10 seconds.
	Timeout time.Duration
	// the URL of the file to download.
	URL string
	// the User-Agent header field value.
	// if the value is empty, the User-Agent header will not be set.
	Useragent string
	// the Referer header field value.
	// if the value is empty, the Referer header will not be set.
	Referer string
	// the maximum number of idle (keep-alive) connections to keep per-host.
	// the default is 16.
	MaxIdleConnsPerHost int
}

type GettingConfig struct {
	// the path to save the downloaded file.
	FilePath string
	// the SHA512 code of the file.
	// if the code is empty, the file will not be checked.
	SHA512 string
	// PartsPath is the path to save the temp files of downloaded parts.
	// if the value is empty, the temp files will be saved in the same directory as the FilePath.
	PartsPath string
	// the name of the part file.
	// if the value is empty, the name will be the same as the file name.
	PartName string
	// the number of parts to download the file.
	// if the value is less than or equal to 0, the file will be downloaded in one part.
	Parts int
	// the progress listener.
	// if the value is nil, the progress will not be listened.
	ListenProgress ProgressListener
}

func (config *RemoteFile) standardize() RemoteFile {
	c := *config

	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.MaxIdleConnsPerHost <= 0 {
		c.MaxIdleConnsPerHost = 16
	}
	if c.Timeout == 0 {
		c.Timeout = time.Duration(10) * time.Second
	}
	return c
}

func (c *GettingConfig) fileName() string {
	_, fileName := filepath.Split(c.FilePath)
	return fileName
}

func (c *GettingConfig) dirPath() string {
	dirPath, _ := filepath.Split(c.FilePath)
	return dirPath
}

func (c *GettingConfig) partFileName(index int) string {
	var fileName string
	if c.Parts == 1 {
		fileName = fmt.Sprintf("%s.downloading", c.PartName)
	} else {
		fileName = fmt.Sprintf("%s.%d.%d.downloading", c.PartName, c.Parts, index)
	}
	return fileName
}

func (config *GettingConfig) standardize() GettingConfig {
	c := *config
	if c.Parts <= 0 {
		c.Parts = 1
	}
	if c.PartName == "" {
		c.PartName = c.fileName()
	}
	if c.PartsPath == "" {
		c.PartsPath = c.dirPath()
	}
	return c
}
