package main

import (
	"bufio"
	//"bytes"
	"context"
	//"crypto/rand"
	//"crypto/rsa"
	"crypto/tls"
	//"crypto/x509"
	//"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	//"math/big"
	"sync"
	"time"

	quic "github.com/lucas-clemente/quic-go"
	logquic "github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"
)

const addr = "test.emes.bj:4448"
const ratio = 1048576

var wg sync.WaitGroup

// Size is needed by the /demo/upload handler to determine the size of the uploaded file
type Size interface {
	Size() int64
}

// Generate data Byte from interger(lengh)
func generatePRData(l int) []byte {
	res := make([]byte, l)
	seed := uint64(1)
	for i := 0; i < l; i++ {
		seed = seed * 48271 % 2147483647
		res[i] = byte(seed)
	}
	return res
}

type bufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}

// NewBufferedWriteCloser creates an io.WriteCloser from a bufio.Writer and an io.Closer
func NewBufferedWriteCloser(writer *bufio.Writer, closer io.Closer) io.WriteCloser {
	return &bufferedWriteCloser{
		Writer: writer,
		Closer: closer,
	}
}

func (h bufferedWriteCloser) Close() error {
	if err := h.Writer.Flush(); err != nil {
		return err
	}
	return h.Closer.Close()
}

var msgSize = 1 << 25 //33MB
var msg = generatePRData(int(msgSize))
var cert, key *string

// We start a server echoing data on the first stream the server opens,
// then connect with a client, send the message, and wait for its receipt.
func main() {
	cert = flag.String("cert", "fullchain.pem", "server cert file for tls config")
	key = flag.String("key", "privkey.pem", "server cert file for tls config")
	flag.Parse()

	fmt.Println("QUIC Testing...")
	quicConf := &quic.Config{
		//MaxIdleTimeout: 60 * time.Second,
	}

	// Qlog setup
	quicConf.Tracer = qlog.NewTracer(func(_ logquic.Perspective, connID []byte) io.WriteCloser {
		//fmt.Println("Setting qlogs...")
		//fmt.Println(connID)
		filename := fmt.Sprintf("server_%s.qlog", time.Now().String())
		f, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Printf("Creating qlog file %s.\n", filename)
		return NewBufferedWriteCloser(bufio.NewWriter(f), f)
	})

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), quicConf)
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println("Listening on ", addr)
	defer listener.Close()
	for true {
		//fmt.Println("Waiting for Test")
		sess, err := listener.Accept(context.Background())
		if err != nil {
			fmt.Println("Session creating error:", err)
			return
		}
		//fmt.Println("Connection Accepted")
		//fmt.Println(sess.ConnectionState())
		//fmt.Println("Server: ", sess.LocalAddr())
		//fmt.Println(sess.RemoteAddr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			//fmt.Println("Waiting for next stream. open by peer..")
			stream, err := sess.AcceptStream(context.Background())
			if err != nil {
				fmt.Println("Stream creating error: ", err)
				return
			}
			//fmt.Println("Stream Accepted with ID: ", stream.StreamID())

			fmt.Println("Download Testing...")
			stream.SetReadDeadline(time.Now().Add(13 * time.Second))
			t1 := time.Now()
			//bytesReceived, err := io.Copy(&buf, stream) //loggingWriter{stream}
			buf := make([]byte, len(msg))
			bytesReceived, _ := io.ReadFull(stream, buf)
			d_temp := time.Since(t1)
			fmt.Println("Bytes Received: ", bytesReceived)
			fmt.Println("Time for receiving", d_temp.Microseconds())
			bps := float64(bytesReceived*8) / d_temp.Seconds()
			Mbps := float64(bps / ratio)
			fmt.Printf("Download Speed: %.3f Mbps", Mbps)
			fmt.Println("")

			fmt.Println("Upload Testing...")
			stream.SetWriteDeadline(time.Now().Add(13 * time.Second))
			bytesSent, _ := stream.Write(msg)
			fmt.Println("Bytes sent:", bytesSent)
			fmt.Println("")

			// sending download stat
			/*d_stat, err := sess.OpenStreamSync(context.Background())
			s := fmt.Sprintf("%.3f", Mbps)
			stream.SetWriteDeadline(time.Now().Add(3 * time.Second))
			bytesSent, _ = stream.Write([]byte(s))*/

		}()
	}
	wg.Wait()
}

// A wrapper for io.Writer that also logs the message.
type loggingWriter struct{ io.Writer }

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Println("Len: ", len(b))
	//fmt.Printf("Server: Got '%s'\n", string(b))
	return w.Writer.Write(b)
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	tlsCert, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
