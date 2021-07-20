package resultwriter

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Result struct {
	Time             time.Time
	UploadDuration   time.Duration
	DownloadDuration time.Duration
	UploadBytes      uint64
	DownloadBytes    uint64
	Source           string
}

type ResultWriter interface {
	Write(result *Result)
	Close()
}

type CSVResultWriter struct {
	output io.WriteCloser
	lock   sync.Mutex
}

func NewCSVResultWriter(filename string) (CSVResultWriter, error) {
	writer, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return CSVResultWriter{writer, sync.Mutex{}}, err
}

func (f *CSVResultWriter) Write(result *Result) {
	uploadDuration := fmt.Sprintf("%s", result.UploadDuration)
	downloadDuration := fmt.Sprintf("%s", result.DownloadDuration)
	downloadBytes := fmt.Sprintf("%d", result.DownloadBytes)
	uploadBytes := fmt.Sprintf("%d", result.UploadBytes)
	time := fmt.Sprintf("%s", result.Time.Format(time.UnixDate))
	output := time + ", " + downloadDuration + ", " + downloadBytes + ", " + uploadDuration + ", " + uploadBytes + ", " + result.Source + "\n"

	f.lock.Lock()
	f.output.Write([]byte(output))
	f.lock.Unlock()
}

func (f *CSVResultWriter) Close() {
	f.output.Close()
}
