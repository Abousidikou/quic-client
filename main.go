package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	quic "github.com/lucas-clemente/quic-go"
)

const ratio = 1048576

var (
	url          = flag.String("u", "emes.bj", "The address to use for getting test done ")
	port         = flag.Int("p", 4447, "The  port to use for getting test done ")
	numberStream = flag.Int("n", 30, "The  number of bidirectional stream ")
	dataSize     = flag.Int("d", 262144, "The  number of bidirectional stream ")
)

//var msgSize = 1 << 25 // 33MB
var mu sync.Mutex

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

// We start a server echoing data on the first stream the client opens,
// then connect with a client, send the message, and wait for its receipt.
func main() {
	//////////////////////   Init
	flag.Parse()
	addr := *url + ":" + strconv.Itoa(*port)
	//downurl := "https://" + *url + ":4444/downloadStat"
	paramUrl := "https://" + *url + ":4444/params?nStream=" + strconv.Itoa(*numberStream) + "&dataSize=" + strconv.Itoa(*dataSize)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	//////////////////////////////////////////////////////////     Parameters
	param, err := http.Get(paramUrl)
	if err != nil {
		return
	}
	body, _ := ioutil.ReadAll(param.Body)
	msgReceived := string(body)
	if msgReceived != "Paramaters Received" {
		fmt.Println("Parameters not received")
		return
	}
	/////////////////  Connection
	quicC := &quic.Config{
		//MaxIdleTimeout: 60 * time.Second,
		MaxIncomingStreams: 150,
	}
	fmt.Println("QUIC Testing")
	sess, err := quic.DialAddr(addr, tlsConf, quicC)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Server : ", sess.RemoteAddr())

	///////////////////////////////////////////////////////////////   Uploading Test
	spin := spinner.New(spinner.CharSets[43], 100*time.Millisecond)
	msg := generatePRData(*dataSize)
	//msgSize := *dataSize / *numberStream
	fmt.Println("Msg Size: ", *dataSize)
	fmt.Println("Nombre de stream: ", *numberStream)
	//spin.FinalMSG = ""
	var w sync.WaitGroup
	//var realMsg []byte
	fmt.Println("Upload Testing")
	spin.Start()
	//var indexByte int
	for i := 0; i < *numberStream; i++ {
		//fmt.Println("Waiting for next stream. Accept by peer..")
		streamUp, err := sess.OpenStreamSync(context.Background())
		if err != nil {
			fmt.Println(" Stream created error: ", err)
		}
		defer streamUp.Close()
		w.Add(1)
		// Ajusting msg size
		//// Start sending
		go func(streamUp quic.Stream) {
			defer w.Done()
			//fmt.Println("file created")
			streamUp.SetWriteDeadline(time.Now().Add(13 * time.Second))
			streamUp.Write(msg)
			//fmt.Println("Bytes sent:" + strconv.Itoa(b))
		}(streamUp)
		//fmt.Println("Go fun lauched with i=", i)
	}
	w.Wait()
	spin.Stop()
	fmt.Println("Upload Complete")
	///////////////////////////////////////////////////////////////////   End Upload Test

	//////////////////////////////////////////////////////////////////     Downloading Test
	fmt.Println("Download Testing")
	var total int
	var times []time.Duration
	spin.Start()
	for i := 0; i < *numberStream; i++ {
		streamDown, err := sess.AcceptStream(context.Background())
		if err != nil {
			fmt.Println(" Stream created error: ", err)
			return
		}
		streamDown.Close()
		//fmt.Println("Stream Accepted with ID: ", streamDown.StreamID())
		w.Add(1)
		go func(streamDown quic.Stream) {
			defer w.Done()
			//fmt.Println("file created")
			//streamDown.SetReadDeadline(time.Now().Add(13 * time.Second))
			t1 := time.Now()
			//bytesReceived, err := io.Copy(&buf, stream) //loggingWriter{stream}
			/*buff := make([]byte, *dataSize)
			byter, _ := io.ReadFull(streamDown, buff)*/
			buff := make([]byte, 4)
			var rByte int
			for {
				streamDown.SetReadDeadline(time.Now().Add(1 * time.Second))
				byter, er := io.ReadFull(streamDown, buff)
				rByte += byter
				if er != nil {
					//fmt.Println(err)
					break
				}
			}
			d_temp := time.Since(t1)
			//fmt.Println("byte: " + strconv.Itoa(byter))
			mu.Lock()
			times = append(times, d_temp)
			//total += byter
			total += rByte
			mu.Unlock()
		}(streamDown)
		//fmt.Println("Go fun lauched with i=", i)
	}
	w.Wait()
	spin.Stop()
	fmt.Println("Download Complete")
	t := times[0]
	for ind := range times {
		if t < times[ind] {
			t = times[ind]
		}
	}
	//fmt.Println("Bytes Received: ", total)
	//fmt.Println("Time for receiving", t.Microseconds())
	bps := float64(total*8) / t.Seconds()
	Mbps := float64(bps / ratio)
	//fmt.Printf("Avg. Download Speed: %.3f Mbps", Mbps)
	//fmt.Println("")
	strMbps := fmt.Sprintf("%.3f", Mbps)
	//fmt.Println("Download Complete")

	////////// Stat
	finishUrl := "https://" + *url + ":4444/testFinished?down=" + strMbps
	res, err := http.Get(finishUrl)
	if err != nil {
		fmt.Println(err)
	}
	bo, _ := ioutil.ReadAll(res.Body)
	sb := string(bo)
	fmt.Println("Results:")
	fmt.Println("         Avg. Download Speed: ", strMbps, " Mbps")
	fmt.Printf("         Avg. Upload Speed: %s Mbps\n", sb)

}
