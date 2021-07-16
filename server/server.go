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

const k = 1024

func main() {
	logger := logging.NewLogger()

	defaultExperimentDuration, _ := time.ParseDuration("10s")
	defaultResultsFile := "output.csv"

	receiverCount := 1
	acceptChannel := make(chan quic.Session, receiverCount)

	serverAddr := flag.String("server", "localhost", "Server hostname or IP address.")
	serverPort := flag.Uint64("port", 4242, "Server port number")
	receiveBufferSize := flag.Uint64("bufferSize", 3*k, "The receive buffer size (subject to kernel limits).")
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
		go func(acceptChannel <-chan quic.Session, serverNumber int) {
			for {
				select {
				case session := <-acceptChannel:

					defer func() {
						logger.Debugf("Server %d is closing its session.\n", serverNumber)
						session.CloseWithError(quic.ApplicationErrorCode(quic.NoError), "")
					}()

					bytesReadArray := make([]byte, 8)
					bytesReadChannel := make(chan uint64)
					buffer := make([]byte, *receiveBufferSize, *receiveBufferSize)

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

					experimentStartTime := time.Now()

					go func(bytesReadOutputChannel chan<- uint64) {
						bytesRead, err := io.ReadFull(dataStream, buffer)
						for err == nil {
							bytesJustRead, errJustSeen := io.ReadFull(dataStream, buffer)
							bytesRead += bytesJustRead
							err = errJustSeen
						}
						bytesReadOutputChannel <- uint64(bytesRead)
					}(bytesReadChannel)

					go func(experimentCompleteChannel <-chan time.Time) {
						<-experimentCompleteChannel
						logger.Debugf("Experiment on server %d ended.\n", serverNumber)
						dataStream.CancelRead(quic.StreamErrorCode(quic.NoError))
						dataStream.Close()
					}(time.After(*experimentDuration))

					bytesRead := <-bytesReadChannel

					experimentEndTime := time.Now()

					logger.Debugf("Experiment on server %d received %d bytes.\n", serverNumber, bytesRead)

					binary.LittleEndian.PutUint64(bytesReadArray, bytesRead)
					controlStream.Write(bytesReadArray)

					controlStream.Close()

					result := resultwriter.Result{Time: time.Now(), Duration: experimentEndTime.Sub(experimentStartTime), Bytes: bytesRead, Source: session.RemoteAddr().String()}
					resultWriter.Write(&result)
				}
			}

		}(acceptChannel, i)
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
