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
	"net/http/httputil"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/op/go-logging"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	command   chan int
	err       chan error
	incoming  chan *http.Response
	lastError error

	waitBeforeUse int
	debug         bool
	logger        *logging.Logger
	stats         *StatLogger
	pi            *MailPingInformation
	urlInfo       *url.URL
	active        bool
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
	for k,v := range ex.pi.HttpHeaders {
		req.Header.Add(k, v)
	}
	if header := req.Header.Get("User-Agent"); header == "" {
		req.Header.Add("User-Agent", "NachoCovePingerv0.9")
	}
	if header := req.Header.Get("Accept"); header == "" {
		req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	
	credentials, err := ex.pi.userCredentials()
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
	if DefaultPollingContext.config.Global.DumpRequests {
		requestBytes, err := httputil.DumpRequest(request, false)
		if err != nil {
			ex.logger.Error("DumpRequest error; %v", err)
		} else {
			ex.logger.Debug("%s: sending request: %s", ex.logPrefix(), requestBytes)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		ex.sendError(err)
		return
	}
	ex.incoming <- response
}

func (ex *ExchangeClient) logPrefix() string {
	return fmt.Sprintf("%s@%s", ex.pi.ClientId, ex.urlInfo.Host)
}

func (ex *ExchangeClient) startLongPoll() {
	defer recoverCrash(ex.logger)
	var logPrefix string = ex.logPrefix()
	ex.logger.Debug("%s: started longpoll", logPrefix)

	deviceInfo, err := getDeviceInfo(DefaultPollingContext.dbm, ex.pi.ClientId)
	if err != nil {
		ex.sendError(err)
		return
	}

	// TODO Can we cache the validation results here? Can they change once a client ID has been invalidated? How do we even invalidate one?
	ex.logger.Debug("%s: Validating clientID", logPrefix)
	err = validateCognitoId(ex.pi.ClientId)
	if err != nil {
		ex.sendError(err)
		return
	}

	if deviceInfo.AWSEndpointArn == "" {
		ex.logger.Debug("%s: Registering %s:%s with AWS.", logPrefix, ex.pi.PushService, ex.pi.PushToken)
		err = deviceInfo.registerAws()
		if err != nil {
			ex.sendError(err)
			return
		}
	} else {
		err = deviceInfo.validateAws()
		if err != nil {
			ex.sendError(err)
			return
		}
	}
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
		Jar:       cookies,
		Transport: tr,
		
	}
	if ex.pi.ResponseTimeout > 0 {
		client.Timeout = time.Duration(ex.pi.ResponseTimeout) * time.Second
	}

	ex.logger.Debug("%s: New HTTP Client with timeout %d %s", logPrefix, ex.pi.ResponseTimeout, ex.pi.MailServerUrl)
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
			responseBody := make([]byte, response.ContentLength)
			_, err := response.Body.Read(responseBody)
			if err != nil && err != io.EOF {
				ex.sendError(err)
				return
			}
			if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
				ex.logger.Debug("%s: response and body: %v %s", logPrefix, *response, responseBody)
			}
			switch {
			case response.StatusCode != 200:
				ex.logger.Debug("%s: Non-200 response: %d", logPrefix, response.StatusCode)
				ex.sendError(errors.New(fmt.Sprintf("Http %d status response", response.StatusCode)))
				return

			case ex.pi.HttpNoChangeReply != nil && bytes.Compare(responseBody, ex.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", logPrefix)

			default:
				newMail := false
				if bytes.Compare(responseBody, ex.pi.HttpExpectedReply) == 0 {
					// there's new mail!
					ex.logger.Debug("%s: Reply matched HttpExpectedReply. Send Push", logPrefix)
					newMail = true
				} else {
					if ex.pi.HttpNoChangeReply != nil {
						// apparently the 'no-change' above didn't match, so this must be a change
						newMail = true
					} else {
						ex.sendError(errors.New(fmt.Sprintf("%s: Unhandled response %v", logPrefix, response)))
						return
					}
				}
				if newMail {
					ex.logger.Debug("%s: Sending push message for new mail", logPrefix)
					err = deviceInfo.push("You've got mail!")
					if err != nil {
						ex.sendError(err)
						return
					}
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
	_, fn, line, _ := runtime.Caller(1)
	ex.err <- errors.New(fmt.Sprintf("%s:%d %s", fn, line, err))
}

func (ex *ExchangeClient) waitForError() {
	select {
	case err := <-ex.err:
		ex.logger.Error(err.Error())
		ex.lastError = err
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
	go ex.waitForError()
	go ex.startLongPoll()
	return nil
}

func (ex *ExchangeClient) Action(action int) error {
	ex.command <- action
	return nil
}

func (ex *ExchangeClient) Status() (MailClientStatus, error) {
	if ex.active {
		return MailClientStatusPinging, nil
	} else {
		return MailClientStatusError, ex.lastError
	}
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
		incoming: make(chan *http.Response),
		command:  make(chan int, 2),
		err:      make(chan error),
		debug:    debug,
		stats:    NewStatLogger(logger),
		logger:   logger,
		active:   true,
	}, nil
}
