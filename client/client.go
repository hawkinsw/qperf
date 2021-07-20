package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hawkinsw/qperf/logging"
	"github.com/hawkinsw/qperf/utils"
	quic "github.com/lucas-clemente/quic-go"
)

const k = 1024

func receive(done <-chan struct{}, bufferSize uint64, stream *quic.Stream, logger *logging.Logger) {
	receiveBuffer := make([]byte, bufferSize, bufferSize)

receiveLoop:
	for {
		select {
		case <-done:
			break receiveLoop
		default:
			_, err := io.ReadFull((*stream), receiveBuffer)
			if err != nil {
				break
			}
		}
	}
	logger.Debugf("Ending receive() go function.\n")
}
func transmit(done <-chan struct{}, bufferSize uint64, stream *quic.Stream, logger *logging.Logger) {
	sendBuffer := make([]byte, bufferSize, bufferSize)
	count, err := io.ReadFull(rand.Reader, sendBuffer)
	if err != nil || count == 0 {
		logger.Errorf("Could not create a %d buffer of random data: %s\n", bufferSize, err)
		os.Exit(1)
	}

transmitLoop:
	for {
		select {
		case <-done:
			break transmitLoop
		default:
			_, err := (*stream).Write(sendBuffer)
			if err != nil {
				break
			}
		}
	}
	logger.Debugf("Ending transmit() go function.\n")
}

func main() {

	logger := logging.NewLogger()

	experimentDone := make(chan struct{})
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	controlBuffer := make([]byte, 8, 8)

	serverAddr := flag.String("server", "localhost", "Server hostname or IP address.")
	serverPort := flag.Uint64("port", 4242, "Server port number")
	readWriteBufferSize := flag.Uint64("readWriteBufferSize", 3*k, "The read/write buffer size.")
	debug := flag.Bool("debug", false, "enable debugging output")
	flag.Parse()

	if *debug {
		logger.SetLogLevel(logging.Debug)
	}

	connection, err := quic.DialAddr(utils.CreateCompleteNetworkAddress(*serverAddr, *serverPort), tlsConf, nil)
	if err != nil {
		logger.Errorf("Could not open a QUIC session to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}

	// Open a QUIC stream to the server on this connection that will be used for uploading/downloading
	// data.
	dataStream, err := connection.OpenStreamSync(context.Background())
	if err != nil {
		logger.Errorf("Could not open a QUIC data stream to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}

	// Open a QUIC stream to the server on this connection that will be used for coordinating the tests
	// and getting results from the server.
	controlStream, err := connection.OpenStreamSync(context.Background())
	if err != nil {
		logger.Errorf("Could not open a QUIC control stream to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}

	// Begin uploading data to the server as fast as possible.
	go transmit(experimentDone, *readWriteBufferSize, &dataStream, &logger)
	// Streams are only opened once data is sent. Write 8 bytes to force the server
	// to accept a control stream.
	controlStream.Write(controlBuffer)

	// Now, wait for the server to send us back a 64-bit number saying how many bytes
	// we sent during the experiment.
	io.ReadFull(controlStream, controlBuffer)
	fmt.Printf("Uploaded: %v bytes\n", binary.LittleEndian.Uint64(controlBuffer))

	// Now, end the stream's transmission to the server.
	dataStream.CancelWrite(quic.StreamErrorCode(quic.NoError))
	experimentDone <- struct{}{}

	// Begin reading data from the server as fast as possible.
	go receive(experimentDone, *readWriteBufferSize, &dataStream, &logger)

	// Now, wait for the server to send us back a 64-bit number saying how many bytes
	// we read during the experiment.
	io.ReadFull(controlStream, controlBuffer)
	fmt.Printf("Downloaded: %v bytes\n", binary.LittleEndian.Uint64(controlBuffer))

	// Now, end the stream's read from the server.
	dataStream.CancelRead(quic.StreamErrorCode(quic.NoError))
	experimentDone <- struct{}{}

	// Close all streams and the connection.
	controlStream.Close()
	dataStream.Close()
	connection.CloseWithError(quic.ApplicationErrorCode(quic.NoError), "")

	os.Exit(0)
}
