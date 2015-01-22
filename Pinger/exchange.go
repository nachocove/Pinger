package Pinger

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/op/go-logging"
)

type Response struct {
	body     []byte
	response http.Response
}

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	command   chan int
	err       chan error
	incoming  chan Response
	lastError error

	waitBeforeUse int
	debug         bool
	logger        *logging.Logger
	stats         *StatLogger
	pi            *MailPingInformation
	urlInfo       *url.URL
}

// String convert the ExchangeClient structure to something printable
func (ex *ExchangeClient) String() string {
	return fmt.Sprintf("Exchange %s", ex)
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomInt(x, y int) int {
	return rand.Intn(y-x) + x
}

func (ex *ExchangeClient) newRequest() (*http.Request, error) {
	requestBody := bytes.NewReader(ex.pi.HttpRequestData)
	req, err := http.NewRequest("POST", ex.pi.MailServerUrl, requestBody)
	if err != nil {
		return nil, err
	}
	credentials, err := ex.pi.UserCredentials()
	if err != nil {
		return nil, err
	}
	username, ok := credentials["Username"]
	if ok == false {
		return nil, errors.New("No Username in credentials")
	}
	password, ok := credentials["Password"]
	if ok == false {
		return nil, errors.New("No Password in credentials")
	}
	req.SetBasicAuth(username, password)
	return req, nil
}

func (ex *ExchangeClient) doRequestResponse(client *http.Client, request *http.Request) {
	response, err := client.Do(request)
	if err != nil {
		ex.sendError(err)
		return
	}
	responseBody := make([]byte, response.ContentLength)
	n, err := response.Body.Read(responseBody)
	ex.logger.Debug("Read %d bytes and error %v", n, err)
	if err != nil && err != io.EOF {
		ex.sendError(err)
		return
	}
	ex.incoming <- Response{body: responseBody, response: *response}
}

func (ex *ExchangeClient) logPrefix() string {
	return fmt.Sprintf("%s@%s", ex.pi.ClientId, ex.urlInfo.Host)
}

func (ex *ExchangeClient) startLongPoll() {
	var logPrefix string = ex.logPrefix()
	ex.logger.Debug("%s: started longpoll", logPrefix)

	if ex.pi.WaitBeforeUse > 0 {
		ex.logger.Debug("%s: WaitBeforeUse %d", logPrefix, ex.pi.WaitBeforeUse)
		time.Sleep(time.Duration(ex.pi.WaitBeforeUse) * time.Second)
	}

	cookies, err := cookiejar.New(nil)
	if err != nil {
		ex.sendError(err)
		return
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ex.debug},
	}
	client := &http.Client{
		Timeout:   time.Duration(ex.pi.ResponseTimeout) * time.Second,
		Jar:       cookies,
		Transport: tr,
	}
	ex.logger.Debug("%s: New HTTP Client with timeout %d", logPrefix, ex.pi.ResponseTimeout)
	stopPolling := false
	var sleepTime time.Duration
	for {
		request, err := ex.newRequest()
		if err != nil {
			ex.sendError(err)
			return
		}
		requestSent := time.Now()
		sleepTime = time.Duration(0)
		go ex.doRequestResponse(client, request)
		ex.logger.Debug("%s: Waiting for response", logPrefix)
		select {
		case response := <-ex.incoming:
			ex.logger.Debug("%s: response body: %s", logPrefix, response.body)
			switch {
			case response.response.StatusCode != 200:
				ex.logger.Debug("%s: Non-200 response: %v", logPrefix, response.response)
				ex.sendError(errors.New(fmt.Sprintf("Go %d status response", response.response.StatusCode)))
				return

			case ex.pi.HttpNoChangeReply != nil && bytes.Compare(response.body, ex.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", logPrefix)

			default:
				if bytes.Compare(response.body, ex.pi.HttpExpectedReply) == 0 {
					// there's new mail!
					ex.logger.Debug("%s: Reply matched HttpExpectedReply. Send Push", logPrefix)
					panic("Not yet implemented")
				} else {
					ex.logger.Debug("%s: Unhandled response %v", logPrefix, response.response)
				}
			}
			sleepTime = (time.Duration(ex.pi.ResponseTimeout) * time.Second) - time.Since(requestSent)

		case cmd := <-ex.command:
			switch {
			case cmd == Stop:
				ex.logger.Debug("%s: got stop command", logPrefix)
				tr.CancelRequest(request)
				resp, ok := <-ex.incoming // wait for it to cancel
				if ok {
					ex.logger.Debug("%s: lagging response %s", logPrefix, resp)
				}
				stopPolling = true
			default:
				ex.logger.Error("Unknown command %d", cmd)
				continue
			}
		}

		if sleepTime.Seconds() > 0.0 {
			ex.logger.Debug("%s: sleeping %fs before next attempt", logPrefix, sleepTime.Seconds())
			time.Sleep(sleepTime)
		}
		if stopPolling == true {
			break
		}
	}
}

func (ex *ExchangeClient) sendError(err error) {
	ex.lastError = err
	ex.logger.Error("Client threw an error: %s", ex.lastError)
	_, fn, line, _ := runtime.Caller(1)
	ex.logger.Error("[error] %s:%d %v", fn, line, ex.lastError)
	ex.err <- err
}
func (ex *ExchangeClient) waitForError() {
	select {
	case <-ex.err:
		ex.command <- Stop
		return
	}
}

func (ex *ExchangeClient) LastError() error {
	return ex.lastError
}

// Listen sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) LongPoll(wait *sync.WaitGroup) error {
	defer RecoverCrash(ex.logger)
	go ex.waitForError()
	go ex.startLongPoll()
	return nil
}

func (ex *ExchangeClient) Action(action int) error {
	ex.command <- action
	return nil
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(mailInfo *MailPingInformation, debug bool, logger *logging.Logger) (*ExchangeClient, error) {
	urlInfo, err := url.Parse(mailInfo.MailServerUrl)
	if err != nil {
		return nil, err
	}
	return &ExchangeClient{
		urlInfo:  urlInfo,
		pi:       mailInfo,
		incoming: make(chan Response),
		command:  make(chan int, 2),
		err:      make(chan error),
		debug:    debug,
		stats:    NewStatLogger(logger),
		logger:   logger,
	}, nil
}
