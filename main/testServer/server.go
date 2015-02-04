package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/nachocove/Pinger/Pinger"
	"github.com/op/go-logging"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

var activeConnections int

// handleConnection Creates channels for incoming data and error, starts a single goroutine, and echoes all data received back.
func handleConnection(conn net.Conn, disconnectTime, tcpKeepAlive time.Duration, TLSconfig *tls.Config) {
	defer conn.Close()
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		logger.Error("Could not grab tcp conn")
		return
	}
	if tcpKeepAlive > 0 {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(tcpKeepAlive)
		logger.Debug("Set TCP-keepalive to %ds\n", tcpKeepAlive)
	} else {
		tc.SetKeepAlive(false)
	}
	if TLSconfig != nil {
		if debug {
			logger.Info("Accepted TLS connection")
		}
		conn = tls.Server(conn, TLSconfig)
	} else {
		if debug {
			logger.Warning("Accepted TCP-only connection")
		}
	}

	inCh := make(chan []byte)
	eCh := make(chan error)
	// Start a goroutine to read from our net connection
	go func(conn net.Conn, ch chan []byte, eCh chan error) {
		data := make([]byte, 512)
		firstTime := true
		for {
			// try to read the data
			_, err := conn.Read(data)
			if err != nil {
				// send an error if it's encountered
				eCh <- err
				return
			}
			if firstTime {
				tlsconn, ok := conn.(*tls.Conn)
				if ok {
					state := tlsconn.ConnectionState()
					if !state.HandshakeComplete {
						eCh <- errors.New("TLS Handshake not completed")
						return
					}
				}
				firstTime = false
			}
			// send data if we read some.
			ch <- data
		}
	}(conn, inCh, eCh)

	remote := conn.RemoteAddr().String()
	logger.Info("%s: Got connection\n", remote)
	activeConnections++

	var timer *time.Timer
	if disconnectTime > 0 {
		timer = time.NewTimer(disconnectTime)
	} else {
		timer = time.NewTimer(time.Duration(1000000 * time.Hour)) // let's call this infinity
	}
	defer timer.Stop()

	// continuously read from the connection
	for {
		var exitLoop = false
		if disconnectTime > 0 {
			logger.Debug("%s: Waiting %d seconds for something to happen\n", remote, disconnectTime/time.Second)
		}
		select {
		// This case means we recieved data on the connection
		case data := <-inCh:
			// just write the data back. We are the ultimate echo.
			if debug {
				logger.Debug("Received data and sending it back: %s\n", string(data))
			}
			n, err := conn.Write(data)
			if err != nil {
				logger.Error("%v\n", err)
			} else {
				logger.Debug("Sent %d bytes\n", n)
			}

		// This case means we got an error and the goroutine has finished
		case err := <-eCh:
			// handle our error then exit for loop
			if err == io.EOF {
				logger.Info("%s: Connection closed\n", remote)
			} else {
				logger.Error("%s: %s\n", remote, err.Error())
			}
			exitLoop = true

		case <-timer.C:
			logger.Info("%s: Timer expired.\n", remote)
			exitLoop = true
		}
		if exitLoop {
			break
		}
	}
	logger.Info("%s: Closing connection\n", remote)
	activeConnections--
}

var debug bool
var verbose bool
var usage = func() {
	fmt.Fprintf(os.Stderr, "USAGE: %s ....\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func memStatsExtraInfo(stats *Pinger.MemStats) string {
	k := float64(1024.0)
	if activeConnections > 0 {
		var allocM = (float64(stats.Memstats.Alloc) - float64(stats.Basememstats.Alloc)) / k
		return fmt.Sprintf("number of connections: %d (est. mem/conn %fk)", activeConnections, allocM/float64(activeConnections))
	}
	return fmt.Sprintf("number of connections: %d", activeConnections)
}

var logger *logging.Logger

func main() {
	var port int
	var help bool
	var minWaitTime int
	var maxWaitTime int
	var logFileName string
	var logFileLevel string
	var certFile string
	var keyFile string
	var certChainFile string
	var bindAddress string
	var printMemPeriodic int
	var tcpKeepAlive int
	var doHttp bool

	flag.IntVar(&port, "p", 8082, "Listen port")
	flag.IntVar(&minWaitTime, "min", 0, "min wait time")
	flag.IntVar(&maxWaitTime, "max", 0, "max wait time")
	flag.StringVar(&logFileName, "log-file", "testServer.log", "log file")
	flag.StringVar(&logFileLevel, "log-level", "WARNING", "Logging level for the logfile (DEBUG, INFO, WARN, NOTICE, ERROR, CRITICAL)")
	flag.StringVar(&bindAddress, "b", "", "bind address")
	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Verbose")
	flag.BoolVar(&doHttp, "http", false, "Handle http requests and responses, instead of raw TCP")
	flag.IntVar(&printMemPeriodic, "mem", 0, "print memory periodically mode in seconds")
	flag.StringVar(&certFile, "cert", "", "TLS server Cert")
	flag.StringVar(&keyFile, "key", "", "TLS server Keypair")
	flag.StringVar(&certChainFile, "chain", "", "TLS server cert chain")
	flag.IntVar(&tcpKeepAlive, "tcpkeepalive", 0, "TCP Keepalive in seconds (0 is disabled)")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}
	if minWaitTime > maxWaitTime {
		fmt.Printf("min must be less than max\n")
		usage()
		os.Exit(1)
	}
	var screenLogging = false
	var screenLevel = logging.ERROR
	if debug || verbose {
		screenLogging = true
		switch {
		case debug:
			screenLevel = logging.DEBUG

		case verbose:
			screenLevel = logging.INFO
		}
	}
	fileLevel, err := logging.LogLevel(logFileLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LevelNameToLevel: %v\n", err)
		os.Exit(1)
	}
	logger, err = Pinger.InitLogging("TestServer", logFileName, fileLevel, screenLogging, screenLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "InitLogging: %v\n", err)
		os.Exit(1)
	}

	var TLSconfig *tls.Config

	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			fmt.Fprintln(os.Stderr, "Need both -cert and -key (and optionally -chain)")
			os.Exit(1)
		}
		logger.Info("Loading cert and key: %s, %s\n", certFile, keyFile)
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read cert and key files")
			os.Exit(1)
		}

		TLSconfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		if certChainFile != "" {
			caCertChain, err := ioutil.ReadFile(certChainFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Open %s: %v\n", certChainFile, err)
				os.Exit(1)
			}
			pool := x509.NewCertPool()
			ok := pool.AppendCertsFromPEM(caCertChain)
			if !ok {
				fmt.Fprintf(os.Stderr, "Could not parse certfile %s\n", certChainFile)
				os.Exit(1)
			}
			TLSconfig.RootCAs = pool
		}
	}

	dialString := fmt.Sprintf("%s:%d", bindAddress, port)

	var memstats *Pinger.MemStats
	if printMemPeriodic > 0 {
		memstats = Pinger.NewMemStats(memStatsExtraInfo, debug, false)
		memstats.PrintMemStatsPeriodic(printMemPeriodic)
	}

	logger.Info("Listening on %s\n", dialString)
	if doHttp {
		http.HandleFunc("/", echoServer)
		if certFile != "" || keyFile != "" {
			http.ListenAndServeTLS(dialString, certFile, keyFile, nil)
		} else {
			http.ListenAndServe(dialString, nil)
		}
	} else {
		addr, err := net.ResolveTCPAddr("tcp", dialString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not resolve address %s: %v\n", dialString, err)
			os.Exit(1)
		}
		addr.Port = port
		TCPLn, err := net.ListenTCP("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not listen in %s: %v\n", dialString, err)
			os.Exit(1)
		}

		logger.Debug("Min %d, Max %d\n", minWaitTime, maxWaitTime)

		if memstats != nil {
			memstats.SetBaseMemStats()
		}
		for {
			conn, err := TCPLn.Accept()
			if err != nil {
				logger.Error("Could not accept connection", err.Error())
				continue
			}
			var disconnectTime time.Duration
			if minWaitTime > 0 || maxWaitTime > 0 {
				disconnectTime = time.Duration(randomInt(minWaitTime, maxWaitTime)) * time.Second
			} else {
				disconnectTime = 0
			}
			tcpKeepAliveDur := time.Duration(tcpKeepAlive) * time.Second

			// this adds 2 goroutines per connection. One the handleConnection itself, which then launches a read-goroutine
			go handleConnection(conn, disconnectTime, tcpKeepAliveDur, TLSconfig)
		}
	}
}

func echoServer(w http.ResponseWriter, r *http.Request) {
	//	body, err := httputil.DumpRequest(r, true)
	//	if err != nil {
	//		http.Error(w, err.Error(), http.StatusInternalServerError)
	//		return
	//	}
	//	logger.Debug("Request: %s", body)
	body := make([]byte, r.ContentLength)
	n, err := r.Body.Read(body)
	if err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Debug("Request body(%d): %s", n, body)
	r.Body.Close()
	fmt.Fprintf(w, "%s", string(body))
}
