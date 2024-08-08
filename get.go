package oget

import (
	"context"
	"time"
)

type OGet struct {
	// the URL of the file to download.
	URL string
	// the path to save the downloaded file.
	FilePath string
	// the context for the http request.
	// if the value is nil, the context will be context.Background().
	Context context.Context
	// the maximum amount of time a dial will wait for a connect or copy to complete.
	// the default is 10 seconds.
	Timeout time.Duration
	// the User-Agent header field value.
	// if the value is empty, the User-Agent header will not be set.
	Useragent string
	// the Referer header field value.
	// if the value is empty, the Referer header will not be set.
	Referer string
	// the maximum number of idle (keep-alive) connections to keep per-host.
	// the default is 16.
	MaxIdleConnsPerHost int
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

func (o OGet) Get() (func() error, error) {
	clean := func() error { return nil }
	task, err := CreateGettingTask(&RemoteFile{
		Context:             o.Context,
		Timeout:             o.Timeout,
		URL:                 o.URL,
		Useragent:           o.Useragent,
		Referer:             o.Referer,
		MaxIdleConnsPerHost: o.MaxIdleConnsPerHost,
	})
	if err != nil {
		return clean, err
	}
	return task.Get(&GettingConfig{
		FilePath:       o.FilePath,
		SHA512:         o.SHA512,
		PartsPath:      o.PartsPath,
		PartName:       o.PartName,
		Parts:          o.Parts,
		ListenProgress: o.ListenProgress,
	})
}
