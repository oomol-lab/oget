# oget

oget is a Golang download library. It supports parallel downloads, resuming after failures, SHA512 verification, and download progress monitoring.

## Installation

```shell
$ go install github.com/oomol-lab/oget
```

## Quick Start

```go
import "github.com/oomol-lab/oget"

_, err := (&OGet{
    URL:      "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
    FilePath: "/path/to/save/file.bin",
}).Get()

if err != nil {
    panic(err)
}
```

## Advanced Features

### Segmented Download

It splits large files into multiple smaller parts for parallel downloading, then merges them into one large file. Additionally, you can specify the temporary directory for storing the split files by adding the `PartsPath` field.

```go
import "github.com/oomol-lab/oget"

_, err := (&OGet{
    URL:       "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
    // Path to store the final file
    FilePath:  "/path/to/save/file.bin",
    // Number of file parts for splitting and parallel downloading
    Parts:     4,
    // If not specified, defaults to the same directory as `FilePath`
    PartsPath: "/path/to/save/temp/files",
}).Get()

if err != nil {
    panic(err)
}
```

### Download Progress Monitoring

`ListenProgress` is not thread-safe and may be called in multiple threads. You need to manually lock it to ensure thread safety.

```go
import "github.com/oomol-lab/oget"

var mux sync.Mutex
_, err := (&OGet{
    URL:            "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
    FilePath:       "/path/to/save/file.bin",
    ListenProgress: func(event oget.ProgressEvent) {
        // Callback may be invoked in multiple threads
        // Use a lock to ensure thread safety
        mux.Lock()
        defer mux.Unlock()
        switch event.phase {
        case oget.ProgressPhaseDownloading:
        // Progress of downloading from the network
        case oget.ProgressPhaseCoping:
        // Download complete, merging multiple file parts into one file
        case oget.ProgressPhaseDone:
        // All tasks are completed
        }
        // Number of bytes completed in this step
        progress := event.Progress
        // Total number of bytes in this step
        total := event.Total
    },
}).Get()

if err != nil {
    panic(err)
}
```

### SHA512 Verification

After downloading, the library performs a SHA512 checksum on the entire file. If the checksum fails, an `oget.SHA512Error` is thrown.

```go
import "github.com/oomol-lab/oget"

_, err := (&OGet{
    URL:      "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
    FilePath: "/path/to/save/file.bin",
    SHA512:    "d286fbb1fab9014fdbc543d09f54cb93da6e0f2c809e62ee0c81d69e4bf58eec44571fae192a8da9bc772ce1340a0d51ad638cdba6118909b555a12b005f2930",
}).Get()

if err != nil {
    if sha512Error, ok := err.(oget.SHA512Error); ok {
        // Failed due to SHA512 verification failure
    }
    panic(err)
}
```

### Resuming Downloads

During a download, oget creates a temporary file with the extension `*.downloading` (regardless of whether it's split into parts). If a download fails due to network issues and the temporary file is not deleted, resuming the download will retain the progress from the previous attempt. To implement resuming downloads, ignore download failures caused by network issues and retry the download.

```go
import "github.com/oomol-lab/oget"
success := false

for i := 0; i < 10; i++ {
    clean, err := (&OGet{
        URL:      "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
        FilePath: "/path/to/save/file.bin",
        Parts:    4,
    }).Get()
    if err != nil {
        if sha512Error, ok := err.(oget.SHA512Error); ok {
            clean()
            panic(sha512Error)
        }
        fmt.Printf("download failed with error and retry %s", err)
    } else {
        success = true
    }
}
if !success {
    panic("download fail")
}
```
The above method will send a request to the server for the file meta each time you retry. If you don't want this meaningless request, you can use the following method.

First, create a `task` object using `oget.CreateGettingTask`. This step only fetches file metadata from the server without starting the download. If this step fails, it should be considered a complete failure of the download task.

```go
import "github.com/oomol-lab/oget"

task, err := oget.CreateGettingTask(&oget.RemoteFile{
    URL: "https://github.com/oomol-lab/oget/raw/main/tests/target.bin",
})
if err != nil {
    panic(err)
}
```

Then, call `task.Get()` to initiate the download. Check if the error is of type `oget.SHA512Error`. If not, it is likely due to network issues and should be retried.

Note that the first return value of `task.Get()` is a function `clean` that deletes the temporary download files. Call it to free up disk space if you don't want to keep these files for the next download attempt after a download failure.

```go
success := false

for i := 0; i < 10; i++ {
    clean, err = task.Get(&oget.GettingConfig{
        FilePath: "/path/to/save/file.bin",
        Parts:    4,
    })
    if err != nil {
        if sha512Error, ok := err.(oget.SHA512Error); ok {
            clean()
            panic(sha512Error)
        }
        fmt.Printf("download failed with error and retry %s", err)
    } else {
        success = true
    }
}
if !success {
    panic("download fail")
}
```