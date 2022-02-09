package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/briandowns/spinner"
	quic "github.com/lucas-clemente/quic-go"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

var (
	url  = flag.String("url", "emes.bj", "The address to use for getting test done ")
	port = flag.Int("p", 4447, "The  port to use for getting test done ")
)

const ratio = 1048576

// A wrapper for io.Writer that also logs the message.
type loggingWriter struct{ io.Writer }

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Printf("Server: Got '%s'\n", string(b))
	return w.Writer.Write(b)
}

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

// We start a server echoing data on the first stream the client opens,
// then connect with a client, send the message, and wait for its receipt.
func main() {
	flag.Parse()
	//
	addr := *url + ":" + strconv.Itoa(*port)
	downurl := "https://" + *url + ":4444/downloadStat"

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	fmt.Println("QUIC Testing")

	//fmt.Println("Establishing session...")
	session, err := quic.DialAddr(addr, tlsConf, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Server : ", session.RemoteAddr())

	//fmt.Println("Opening unidirectional stream...")
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer stream.Close()
	//fmt.Println("Stream Opened")
	spin := spinner.New(spinner.CharSets[43], 100*time.Millisecond)
	//spin.FinalMSG = ""
	fmt.Println("Upload Testing")
	stream.SetWriteDeadline(time.Now().Add(13 * time.Second))
	spin.Start()
	stream.Write(msg)
	spin.Stop()
	//fmt.Println("BytesSent: ", bytesSent)
	resp, err := http.Get(downurl)
	if err == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		//Convert the body to type string
		sb := string(body)
		fmt.Printf("Avg. Upload Speed: %s Mbps\n", sb)
	}
	fmt.Println("Upload Complete")
	fmt.Println("")
	fmt.Println("Download Testing")
	buf := make([]byte, len(msg))
	stream.SetReadDeadline(time.Now().Add(13 * time.Second))
	t1 := time.Now()
	spin.Restart()
	bytesReceived, _ := io.ReadFull(stream, buf)
	sendTime := time.Since(t1)
	spin.Stop()
	bps := float64(bytesReceived*8) / sendTime.Seconds()
	Mbps := float64(bps / ratio)
	fmt.Printf("Avg. Download Speed: %.3f Mbps", Mbps)
	fmt.Println("")
	fmt.Println("Download Complete")

}
