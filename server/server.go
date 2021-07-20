package main

import (
	"context"
	"encoding/binary"
	"flag"
	"io"
	"os"
	"time"

	"github.com/hawkinsw/qperf/logging"
	"github.com/hawkinsw/qperf/resultwriter"
	"github.com/hawkinsw/qperf/tlsconfig"
	"github.com/hawkinsw/qperf/utils"
	quic "github.com/lucas-clemente/quic-go"
)

type serverID int

const k = 1024

func uploadExperiment(
	bytesReadOutputChannel chan<- uint64,
	dataStream *quic.Stream,
	buffer []byte,
	experimentDuration time.Duration,
	logger *logging.Logger,
	serverNumber serverID) {

	go func() {
		bytesRead, err := io.ReadFull(*dataStream, buffer)
		for err == nil {
			bytesJustRead, errJustSeen := io.ReadFull(*dataStream, buffer)
			bytesRead += bytesJustRead
			err = errJustSeen
		}
		bytesReadOutputChannel <- uint64(bytesRead)
	}()

	go func(experimentCompleteChannel <-chan time.Time) {
		<-experimentCompleteChannel
		logger.Debugf("Upload experiment on server %d ended.\n", serverNumber)
		(*dataStream).CancelRead(quic.StreamErrorCode(quic.NoError))
	}(time.After(experimentDuration))

}

func downloadExperiment(
	bytesWrittenOutputChannel chan<- uint64,
	dataStream *quic.Stream,
	buffer []byte,
	experimentDuration time.Duration,
	logger *logging.Logger,
	serverNumber serverID) {
	go func() {
		bytesWritten, err := (*dataStream).Write(buffer)
		for err == nil {
			bytesJustWritten, errJustSeen := (*dataStream).Write(buffer)
			bytesWritten += bytesJustWritten
			err = errJustSeen
		}
		bytesWrittenOutputChannel <- uint64(bytesWritten)
	}()

	go func(experimentCompleteChannel <-chan time.Time) {
		<-experimentCompleteChannel
		logger.Debugf("Download experiment on server %d ended.\n", serverNumber)
		(*dataStream).CancelWrite(quic.StreamErrorCode(quic.NoError))
	}(time.After(experimentDuration))
}

func main() {
	logger := logging.NewLogger()

	defaultExperimentDuration, _ := time.ParseDuration("10s")
	defaultResultsFile := "output.csv"

	receiverCount := 1
	acceptChannel := make(chan quic.Session, receiverCount)

	serverAddr := flag.String("server", "localhost", "Server hostname or IP address.")
	serverPort := flag.Uint64("port", 4242, "Server port number")
	readWriteBufferSize := flag.Uint64("readWriteBufferSize", 3*k, "The read/write buffer size (subject to kernel limits).")
	experimentDuration := flag.Duration("experimentDuration", defaultExperimentDuration, "length of each experiment")
	resultsFile := flag.String("resultsFile", defaultResultsFile, "Name of file to store results")
	debug := flag.Bool("debug", false, "enable debugging output")
	flag.Parse()

	if *debug {
		logger.SetLogLevel(logging.Debug)
	}

	resultWriter, err := resultwriter.NewCSVResultWriter(*resultsFile)
	if err != nil {
		logger.Errorf("Could not open %s for writing results.\n", *resultsFile)
		os.Exit(1)
	}
	defer func() {
		logger.Debugf("Closing the result writer.\n")
		resultWriter.Close()
	}()

	listener, err := quic.ListenAddr(utils.CreateCompleteNetworkAddress(*serverAddr, *serverPort), tlsconfig.GenerateTLSConfig(), nil)
	if err != nil {
		logger.Errorf("Could not start the server on %s:%d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}

	for i := 0; i < receiverCount; i++ {
		go func(acceptChannel <-chan quic.Session, serverNumber serverID) {
			for {
				select {
				case session := <-acceptChannel:

					defer func() {
						logger.Debugf("Server %d is closing its session.\n", serverNumber)
						session.CloseWithError(quic.ApplicationErrorCode(quic.NoError), "")
					}()

					bytesReadArray := make([]byte, 8)
					bytesReadChannel := make(chan uint64)
					bytesWrittenArray := make([]byte, 8)
					bytesWrittenChannel := make(chan uint64)
					buffer := make([]byte, *readWriteBufferSize, *readWriteBufferSize)

					dataStream, err := session.AcceptStream(context.Background())
					if err != nil {
						logger.Errorf("Server %d could not accept the data stream: %v\n", serverNumber, err)
						break
					}
					logger.Debugf("Server %d opened a data stream\n", serverNumber)
					controlStream, err := session.AcceptStream(context.Background())
					if err != nil {
						logger.Errorf("Server %d could not accept the control stream: %v\n", serverNumber, err)
						dataStream.Close()
						break
					}
					logger.Debugf("Server %d opened a control stream\n", serverNumber)

					// Start the upload test.

					experimentStartTime := time.Now()
					uploadExperiment(bytesReadChannel, &dataStream, buffer, *experimentDuration, &logger, serverNumber)
					bytesRead := <-bytesReadChannel
					experimentEndTime := time.Now()
					uploadExperimentDuration := experimentEndTime.Sub(experimentStartTime)
					logger.Debugf("Upload experiment on server %d received %d bytes.\n", serverNumber, bytesRead)
					binary.LittleEndian.PutUint64(bytesReadArray, bytesRead)
					controlStream.Write(bytesReadArray)

					// Start the download test.

					experimentStartTime = time.Now()
					downloadExperiment(bytesWrittenChannel, &dataStream, buffer, *experimentDuration, &logger, serverNumber)
					bytesWritten := <-bytesWrittenChannel
					experimentEndTime = time.Now()
					downloadExperimentDuration := experimentEndTime.Sub(experimentStartTime)

					logger.Debugf("Download experiment on server %d transmitted %d bytes.\n", serverNumber, bytesWritten)

					binary.LittleEndian.PutUint64(bytesWrittenArray, bytesWritten)
					controlStream.Write(bytesWrittenArray)

					dataStream.Close()
					controlStream.Close()

					result := resultwriter.Result{
						Time:             time.Now(),
						UploadDuration:   uploadExperimentDuration,
						DownloadDuration: downloadExperimentDuration,
						UploadBytes:      bytesRead,
						DownloadBytes:    bytesWritten,
						Source:           session.RemoteAddr().String(),
					}
					resultWriter.Write(&result)
				}
			}

		}(acceptChannel, serverID(i))
	}

	for {
		logger.Debugf("Waiting to accept a connection from a client.\n")
		sess, err := listener.Accept(context.Background())
		if err != nil {
			return
		}
		acceptChannel <- sess
		logger.Debugf("Accepted a connection from a client.\n")
	}
}
