package resultwriter

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Result struct {
	Time     time.Time
	Duration time.Duration
	Bytes    uint64
	Source   string
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
	duration := fmt.Sprintf("%s", result.Duration)
	bytes := fmt.Sprintf("%d", result.Bytes)
	time := fmt.Sprintf("%s", result.Time.Format(time.UnixDate))
	output := time + ", " + duration + ", " + result.Source + ", " + bytes + "\n"

	f.lock.Lock()
	f.output.Write([]byte(output))
	f.lock.Unlock()
}

func (f *CSVResultWriter) Close() {
	f.output.Close()
}
