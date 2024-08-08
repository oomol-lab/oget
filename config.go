package oget

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

type RemoteFile struct {
	Context             context.Context
	Timeout             time.Duration
	URL                 string
	Useragent           string
	Referer             string
	MaxIdleConnsPerHost int
}

type GettingConfig struct {
	FilePath       string
	SHA512         string
	PartsPath      string
	PartName       string
	Parts          int
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
