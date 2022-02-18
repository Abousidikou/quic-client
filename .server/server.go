package main

import (
	"bufio"
	"strconv"

	//"bytes"
	"context"
	//"crypto/rand"
	//"crypto/rsa"
	"crypto/tls"
	//"crypto/x509"
	//"encoding/pem"
	"encoding/json"
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
var mu sync.Mutex
var msgSize = 1 << 25 //33MB
var cert, key *string

// Size is needed by the /demo/upload handler to determine the size of the uploaded file
type Size interface {
	Size() int64
}
type bufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}
type Params struct {
	numberStream int `json:"numberStream"`
	dataSize     int `json:"dataSize"`
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
	fmt.Println("About to listen on ", addr)
	listener, err := quic.ListenAddr(addr, generateTLSConfig(), quicConf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Listening on ", addr)
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
		//Each routine for each client
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream, err := sess.AcceptStream(context.Background())
			if err != nil {
				fmt.Println("Stream creating error: ", err)
				return
			}
			fmt.Println("Stream Accepted with ID: ", stream.StreamID())
			///////////////   Getting test parameters
			buf := make([]byte, 2)
			var params []byte
			fmt.Println("Getting parameters")
			for {
				if len(params) != 0 {
					stream.SetReadDeadline(time.Now().Add(2 * time.Second))
				}
				n, err := io.ReadFull(stream, buf)
				fmt.Println("Read 2 bytes into buf:", n)
				if err != nil {
					if err == io.EOF {
						fmt.Println(string(buf[:n])) //should handle any remainding bytes.
						params = append(params, buf...)
						break
					}
					fmt.Println(err)
					params = append(params, buf...)
					break
				}

				//fmt.Println("1024 bytes")
				params = append(params, buf...)
				//fmt.Println("Received Bytes: ", len(receivedFileByte), " bytes")
			}
			fmt.Println("Parametre getted")
			fmt.Println("params:", string(params))
			///// Sending Response
			stream.SetWriteDeadline(time.Now().Add(2 * time.Second))
			stream.Write([]byte("FIN"))
			///// Processing parameters
			var p Params
			json.Unmarshal(params, &p)
			fmt.Println("Parameters: ", p)
			if p.numberStream == 0 {
				// valeur par defaut
				p.numberStream = 1
				p.dataSize = msgSize
			}

			fmt.Println("Parameters: ", p)

			fmt.Println("Upload Testing...")
			var total int
			var times []time.Duration
			var w sync.WaitGroup
			for i := 0; i < p.numberStream; i++ {
				fmt.Println(i)
				fmt.Println("Waiting for next stream. open by peer..")
				streamUp, err := sess.AcceptStream(context.Background())
				if err != nil {
					fmt.Println(" Stream created error: ", err)
					return
				}
				fileLog := fmt.Sprintf("fileLog_%d.log", int64(streamUp.StreamID()))
				file, err := os.Create(fileLog)

				defer file.Close()

				if err != nil {
					log.Fatal(err)
				}
				file.WriteString("Stream Accepted with i: " + strconv.Itoa(i) + "\n")
				// each go routine for each stream
				w.Add(1)
				go func(streamUp quic.Stream) {
					defer w.Done()
					//fmt.Println("file created")
					file.WriteString("File created\n")
					streamUp.SetReadDeadline(time.Now().Add(13 * time.Second))
					t1 := time.Now()
					//bytesReceived, err := io.Copy(&buf, stream) //loggingWriter{stream}
					buff := make([]byte, p.dataSize)
					byter, _ := io.ReadFull(streamUp, buff)
					d_temp := time.Since(t1)
					fmt.Println("Bytes reveived :" + strconv.Itoa(byter))
					file.WriteString("Byte received :" + strconv.Itoa(byter) + "\n within time: " + d_temp.String() + "\n")
					mu.Lock()
					times = append(times, d_temp)
					total += byter
					mu.Unlock()
				}(streamUp)
				fmt.Println("Go fun lauched with i=", i)
			}
			w.Wait()
			t := times[0]
			for ind := range times {
				if t < times[ind] {
					t = times[ind]
				}
			}
			fmt.Println("Bytes Received: ", total)
			fmt.Println("Time for receiving", t.Microseconds())
			bps := float64(total*8) / t.Seconds()
			Mbps := float64(bps / ratio)
			fmt.Printf("Upload Speed: %.3f Mbps", Mbps)
			fmt.Println("")

			fmt.Println("Download Testing...")
			msg := generatePRData(p.dataSize)
			//var realMsg []byte
			//var indexByte int
			var bytesSents int
			for i := 0; i < p.numberStream; i++ {
				fmt.Println(i)
				streamDown, err := sess.OpenStreamSync(context.Background())
				if err != nil {
					fmt.Println(" Stream created: ", err)
					return
				}
				fmt.Println("Stream Accepted with ID: ", streamDown.StreamID())
				/*if i == 0 {
					realMsg = msg[:msgSize]
					indexByte = msgSize
				} else if i == p.numberStream-1 {
					realMsg = msg[indexByte:]
				} else {
					realMsg = msg[indexByte : indexByte+msgSize]
					indexByte += msgSize
				}*/
				w.Add(1)
				go func() {
					defer w.Done()
					streamDown.SetWriteDeadline(time.Now().Add(13 * time.Second))
					bytesSent, _ := streamDown.Write(msg)
					fmt.Println("Byte sent:", bytesSent)
					bytesSents += bytesSent
				}()
				fmt.Println("Go func lauched i=", i)
			}
			w.Wait()

			// sending download stat
			fmt.Println("Bytes Sents: ", bytesSents)
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
