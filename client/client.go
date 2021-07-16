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

	"github.com/hawkinsw/qperf/utils"
	quic "github.com/lucas-clemente/quic-go"
)

const k = 1024

func transmit(done <-chan struct{}, bufferSize uint64, stream *quic.Stream) {
	sendBuffer := make([]byte, bufferSize, bufferSize)
	count, err := io.ReadFull(rand.Reader, sendBuffer)
	if err != nil || count == 0 {
		fmt.Printf("Could not create a %d buffer of random data: %s\n", bufferSize, err)
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
				break transmitLoop
			}
		}
	}
}

func main() {

	experimentDone := make(chan struct{})
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	serverAddr := flag.String("server", "localhost", "Server hostname or IP address.")
	serverPort := flag.Uint64("port", 4242, "Server port number")
	sendBufferSize := flag.Uint64("bufferSize", 3*k, "The write buffer size.")
	flag.Parse()

	session, err := quic.DialAddr(utils.CreateCompleteNetworkAddress(*serverAddr, *serverPort), tlsConf, nil)
	if err != nil {
		fmt.Printf("Could not open a QUIC session to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}

	dataStream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Printf("Could not open a QUIC data stream to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}
	controlStream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Printf("Could not open a QUIC control stream to %s on port %d: %s\n", *serverAddr, *serverPort, err)
		os.Exit(1)
	}
	controlBuffer := make([]byte, 8, 8)

	go transmit(experimentDone, *sendBufferSize, &dataStream)
	// Streams are only opened once data is sent. Write 8 bytes to force the server
	// to accept a control stream.
	controlStream.Write(controlBuffer)

	// Now, wait for the server to send us back a 64-bit number saying how many bytes
	// we transferred during the experiment.
	io.ReadFull(controlStream, controlBuffer)
	fmt.Printf("control stream's buffer: %v\n", binary.LittleEndian.Uint64(controlBuffer))

	controlStream.Close()
	dataStream.Close()
	session.CloseWithError(quic.ApplicationErrorCode(quic.NoError), "")

	os.Exit(0)
}
