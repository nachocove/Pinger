package Pinger

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/nachocove/Pinger/Utils"
	"github.com/op/go-logging"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	command       chan PingerCommand
	errCh         chan error
	incoming      chan *http.Response
	stopCh        chan int
	transport     *http.Transport
	request       *http.Request
	client        *http.Client
	lastError     error
	waitBeforeUse int
	debug         bool
	logger        *logging.Logger
	stats         *Utils.StatLogger
	pi            *MailPingInformation
	urlInfo       *url.URL
	is_active     bool
	deviceInfo    *DeviceInfo
	mutex         *sync.Mutex
}

// TODO Need to refactor mailClient.go and exchange.go. A lot of the functionality belongs
// in the parent, and only very few things into this file. That would likely also remove a
// lot of the redundancy in the MailPingInformation and ExchangeClient structures

// NewExchangeClient set up a new exchange client
func NewExchangeClient(mailInfo *MailPingInformation, deviceInfo *DeviceInfo, debug, doStats bool, logger *logging.Logger) (*ExchangeClient, error) {
	urlInfo, err := url.Parse(mailInfo.MailServerUrl)
	if err != nil {
		return nil, err
	}
	ex := &ExchangeClient{
		urlInfo:    urlInfo,
		pi:         mailInfo,
		incoming:   make(chan *http.Response),
		command:    make(chan PingerCommand, 10),
		errCh:      make(chan error),
		stopCh:     make(chan int),
		debug:      debug,
		stats:      nil,
		logger:     logger,
		is_active:  false,
		mutex:      &sync.Mutex{},
		deviceInfo: deviceInfo,
	}
	if doStats {
		ex.stats = Utils.NewStatLogger(ex.stopCh, logger, false)
	}
	return ex, nil
}

func (ex *ExchangeClient) newRequest() (*http.Request, error) {
	requestBody := bytes.NewReader(ex.pi.HttpRequestData)
	req, err := http.NewRequest("POST", ex.pi.MailServerUrl, requestBody)
	if err != nil {
		return nil, err
	}
	for k, v := range ex.pi.HttpHeaders {
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

	req.SetBasicAuth(ex.pi.MailServerCredentials.Username, ex.pi.MailServerCredentials.Password)
	return req, nil
}

func (ex *ExchangeClient) getLogPrefix() string {
	return ex.deviceInfo.getLogPrefix()
}

func (ex *ExchangeClient) doRequestResponse() {
	var err error
	ex.logger.Debug("%s: New HTTP Client with timeout %d msec %s", ex.getLogPrefix(), ex.pi.ResponseTimeout, ex.pi.MailServerUrl)
	ex.request, err = ex.newRequest()
	if err != nil {
		ex.sendError(err)
		return
	}
	if DefaultPollingContext.config.Global.DumpRequests {
		requestBytes, err := httputil.DumpRequest(ex.request, true)
		if err != nil {
			ex.logger.Error("%s: DumpRequest error; %v", ex.getLogPrefix(), err)
		} else {
			ex.logger.Debug("%s: sending request:\n%s", ex.getLogPrefix(), requestBytes)
		}
	}
	response, err := ex.client.Do(ex.request)
	if err != nil {
		ex.sendError(err)
		return
	}
	if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
		responseBytes, err := httputil.DumpResponse(response, true)
		cached_data := ioutil.NopCloser(bytes.NewReader(responseBytes))
		response.Body.Close()
		response.Body = cached_data
		if err != nil {
			ex.logger.Error("%s: Could not dump response %+v", ex.getLogPrefix(), response)
		} else {
			ex.logger.Debug("%s: response:\n%s", ex.getLogPrefix(), responseBytes)
		}
	}
	if ex.incoming != nil {
		ex.incoming <- response
	} else {
		response.Body.Close()
	}
}

func (ex *ExchangeClient) startLongPoll() {
	defer recoverCrash(ex.logger)
	defer ex.pi.SelfDelete() // delete the 'bag'
	ex.is_active = true
	ex.mutex.Unlock()

	ex.logger.Debug("%s: Starting deferTimer for %d msecs", ex.getLogPrefix(), ex.pi.WaitBeforeUse)
	deferTimer := time.NewTimer(time.Duration(ex.pi.WaitBeforeUse) * time.Millisecond)

forLoop:
	for {
		select {
		case <-deferTimer.C:
			ex.logger.Debug("%s: DeferTimer expired. Starting Polling.", ex.getLogPrefix())
			ex.mutex.Lock()
			go ex.run() // will unlock mutex, when the stop channel is initialized. Prevents race-condition where we get a Stop/Defer just as the go routine is starting

		case err := <-ex.errCh:
			ex.lastError = err
			ex.logger.Debug("%s: Stopping goroutines", ex.getLogPrefix())
			ex.Action(PingerStop)

		case cmd := <-ex.command:
			switch {
			case cmd == PingerStop:
				ex.logger.Debug("%s: got 'stop' command", ex.getLogPrefix())
				deferTimer.Stop()
				close(ex.stopCh)
				break forLoop

			case cmd == PingerDefer:
				ex.logger.Debug("%s: reStarting deferTimer for %d msecs", ex.getLogPrefix(), ex.pi.WaitBeforeUse)
				ex.Action(PingerStop)
				deferTimer.Reset(time.Duration(ex.pi.WaitBeforeUse) * time.Millisecond)

			default:
				ex.logger.Error("%s: Unknown command %d", ex.getLogPrefix(), cmd)
				continue

			}
		}
	}
}

func (ex *ExchangeClient) run() {
	defer recoverCrash(ex.logger)
	defer func() {
		ex.Action(PingerStop)
	}()
	ex.mutex.Unlock()

	cookies, err := cookiejar.New(nil)
	if err != nil {
		ex.sendError(err)
		return
	}
	ex.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ex.debug},
	}

	ex.client = &http.Client{
		Jar:       cookies,
		Transport: ex.transport,
	}
	if ex.pi.ResponseTimeout > 0 {
		ex.client.Timeout = time.Duration(ex.pi.ResponseTimeout) * time.Millisecond
	}
	ex.logger.Debug("%s: Starting maxPollTimer for %d msecs", ex.getLogPrefix(), ex.pi.WaitBeforeUse)
	maxPollTimer := time.NewTimer(time.Duration(ex.pi.MaxPollTimeout) * time.Millisecond)
	defer maxPollTimer.Stop()

	for {
		go ex.doRequestResponse()
		ex.logger.Debug("%s: Waiting for response", ex.getLogPrefix())
		select {
		case response := <-ex.incoming:
			responseBody, err := ioutil.ReadAll(response.Body)
			if err != nil {
				ex.sendError(err)
				response.Body.Close() // attempt to close. Ignore any errors.
				return
			}
			err = response.Body.Close()
			if err != nil {
				ex.sendError(err)
				return
			}
			switch {
			case response.StatusCode != 200:
				ex.logger.Debug("%s: Non-200 response: %d", ex.getLogPrefix(), response.StatusCode)
				// ask the client to re-register, since something is bad.
				err = ex.deviceInfo.push(PingerNotificationRegister)
				if err != nil {
					// don't bother with this error. The real/main error is the http status. Just log it.
					ex.logger.Error("%s: Push failed but ignored: %s", ex.getLogPrefix(), err.Error())
				}
				ex.sendError(fmt.Errorf("Http %d status response", response.StatusCode))
				return

			case ex.pi.HttpNoChangeReply != nil && bytes.Compare(responseBody, ex.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", ex.getLogPrefix())

			case ex.pi.HttpExpectedReply == nil || bytes.Compare(responseBody, ex.pi.HttpExpectedReply) == 0:
				// there's new mail!
				ex.logger.Debug("%s: Sending push message for new mail", ex.getLogPrefix())
				err = ex.deviceInfo.push(PingerNotificationNewMail)
				if err != nil {
					if DefaultPollingContext.config.Global.IgnorePushFailure == false {
						ex.sendError(err)
						return
					} else {
						ex.logger.Warning("%s: Push failed but ignored: %s", ex.getLogPrefix(), err.Error())
					}
				}
				return

			default:
				ex.sendError(fmt.Errorf("%s: Unhandled response %v", ex.getLogPrefix(), response))
				return

			}

		case <-ex.stopCh:
			ex.logger.Debug("%s: Stopping", ex.getLogPrefix())
			if ex.request != nil {
				ex.transport.CancelRequest(ex.request)
			}
			resp, ok := <-ex.incoming // wait for it to cancel
			if ok {
				responseBytes, err := httputil.DumpResponse(resp, true)
				if err != nil {
					ex.logger.Error("Could not dump response: %s", err.Error())
				} else {
					ex.logger.Debug("%s: lagging response:\n%s", ex.getLogPrefix(), responseBytes)
				}
			}
			return

		case <-maxPollTimer.C:
			ex.logger.Debug("maxPollTimer expired. Stopping everything.")
			return
		}
	}

}

func (ex *ExchangeClient) sendError(err error) {
	_, fn, line, _ := runtime.Caller(1)
	ex.logger.Error("%s: %s/%s:%d %s", ex.getLogPrefix(), path.Base(path.Dir(fn)), path.Base(fn), line, err)
	ex.errCh <- err
}

// MailClient Interface

// LongPoll sets up the exchange client
func (ex *ExchangeClient) LongPoll(wait *sync.WaitGroup) error {
	if ex.stats != nil {
		go ex.stats.TallyResponseTimes()
	}
	ex.mutex.Lock()
	go ex.startLongPoll()
	return nil
}

// Action sends a command to the go routine.
func (ex *ExchangeClient) Action(action PingerCommand) error {
	ex.mutex.Lock()
	defer ex.mutex.Unlock()

	if ex.is_active == false {
		return errors.New("Not Polling")
	}

	ex.command <- action
	return nil
}

// Status gets the status of the go routine
func (ex *ExchangeClient) Status() (MailClientStatus, error) {
	if ex.is_active {
		return MailClientStatusPinging, nil
	} else {
		return MailClientStatusError, ex.lastError
	}
}

// SelfDelete Used to zero out the structure and let garbage collection reap the empty stuff.
func (ex *ExchangeClient) SelfDelete() {
	ex.logger.Debug("%s: Cleaning up ExchangeClient struct", ex.getLogPrefix())
	// do NOT free or delete ex.pi. We (this function) is called from the pi deletion.
	if ex.deviceInfo != nil {
		ex.deviceInfo.selfDelete()
		ex.deviceInfo = nil // let garbage collector handle it.
	}
}
