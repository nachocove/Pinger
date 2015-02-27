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
	command  chan PingerCommand
	err      chan error
	incoming chan *http.Response
	stopCh   chan PingerCommand

	lastError     error
	waitBeforeUse int
	debug         bool
	logger        *logging.Logger
	stats         *Utils.StatLogger
	pi            *MailPingInformation
	urlInfo       *url.URL
	active        bool
	deviceInfo    *DeviceInfo
	logPrefix     string
	mutex         *sync.Mutex
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

func (ex *ExchangeClient) doRequestResponse(client *http.Client, request *http.Request) {
	if DefaultPollingContext.config.Global.DumpRequests {
		requestBytes, err := httputil.DumpRequest(request, true)
		if err != nil {
			ex.logger.Error("%s: DumpRequest error; %v", ex.getLogPrefix(), err)
		} else {
			ex.logger.Debug("%s: sending request:\n%s", ex.getLogPrefix(), requestBytes)
		}
	}
	response, err := client.Do(request)
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

func (ex *ExchangeClient) getLogPrefix() string {
	if ex.logPrefix == "" {
		ex.logPrefix = fmt.Sprintf("%s@%s", ex.pi.ClientId, ex.urlInfo.Host)
	}
	return ex.logPrefix
}

func (ex *ExchangeClient) validateClient(deviceInfo *DeviceInfo) error {
	// TODO Can we cache the validation results here? Can they change once a client ID has been invalidated? How do we even invalidate one?
	ex.logger.Debug("%s: Validating clientID", ex.getLogPrefix())
	err := ex.pi.validateClientId()
	if err != nil {
		return err
	}

	if deviceInfo.AWSEndpointArn == "" {
		ex.logger.Debug("%s: Registering %s:%s with AWS.", ex.getLogPrefix(), ex.pi.PushService, ex.pi.PushToken)
		err = deviceInfo.registerAws()
		if err != nil {
			if DefaultPollingContext.config.Global.IgnorePushFailure == false {
				return err
			} else {
				ex.logger.Warning("%s: Registering %s:%s error (ignored): %s", ex.getLogPrefix(), ex.pi.PushService, ex.pi.PushToken, err.Error())
			}
		} else {
			ex.logger.Debug("%s: endpoint created %s", ex.getLogPrefix(), deviceInfo.AWSEndpointArn)
		}
		// TODO We should send a test-ping here, so we don't find out the endpoint is unreachable later.
		// It's optional (we'll find out eventually), but this would speed it up.
	} else {
		// Validate this even if the device is marked as deviceInfo.Enabled=false, because this might
		// mark it as enabled again. Possibly...
		err = deviceInfo.validateAws()
		if err != nil {
			if DefaultPollingContext.config.Global.IgnorePushFailure == false {
				return err
			} else {
				ex.logger.Warning("%s: Validating %s:%s error (ignored): %s", ex.getLogPrefix(), ex.pi.PushService, ex.pi.PushToken, err.Error())
			}
		} else {
			ex.logger.Debug("%s: endpoint validated %s", ex.getLogPrefix(), deviceInfo.AWSEndpointArn)
		}
	}
	ex.deviceInfo = deviceInfo
	return nil
}

func (ex *ExchangeClient) startLongPoll() {
	defer recoverCrash(ex.logger)
	ex.command = make(chan PingerCommand, 10)
	defer func() {
		ex.command = nil
		ex.active = false
	}()
	ex.mutex.Unlock()

	ex.logger.Debug("%s: started longpoll", ex.getLogPrefix())

	deviceInfo, err := getDeviceInfo(DefaultPollingContext.dbm, ex.pi.ClientId)
	if err != nil {
		ex.sendError(err)
		return
	}

	err = ex.validateClient(deviceInfo)
	if err != nil {
		ex.sendError(err)
		return
	}

	ex.logger.Debug("%s: Starting deferTimer for %d msecs", ex.getLogPrefix(), ex.pi.WaitBeforeUse)
	deferTimer := time.NewTimer(time.Duration(ex.pi.WaitBeforeUse) * time.Millisecond)

forLoop:
	for {
		select {
		case <-deferTimer.C:
			ex.logger.Debug("DeferTimer expired. Starting Polling.")
			ex.mutex.Lock()
			go ex.run() // will unlock mutex, when the stop channel is initialized. Prevents race-condition where we get a Stop/Defer just as the go routine is starting

		case cmd := <-ex.command:
			switch {
			case cmd == PingerStop:
				ex.logger.Debug("%s: got 'stop' command", ex.getLogPrefix())
				deferTimer.Stop()
				ex.stop()
				break forLoop

			case cmd == PingerDefer:
				ex.logger.Debug("%s: reStarting deferTimer for %d msecs", ex.getLogPrefix(), ex.pi.WaitBeforeUse)
				ex.stop()
				deferTimer.Reset(time.Duration(ex.pi.WaitBeforeUse) * time.Millisecond)

			default:
				ex.logger.Error("%s: Unknown command %d", ex.getLogPrefix(), cmd)
				continue

			}
		}
	}
}

func (ex *ExchangeClient) stop() {
	ex.mutex.Lock()
	defer ex.mutex.Unlock()
	if ex.stopCh != nil {
		ex.stopCh <- PingerStop
	}
}

func (ex *ExchangeClient) run() {
	defer recoverCrash(ex.logger)
	ex.stopCh = make(chan PingerCommand, 2)
	defer func() {
		ex.stopCh = nil
		ex.incoming = nil
	}()
	ex.mutex.Unlock()

	cookies, err := cookiejar.New(nil)
	if err != nil {
		ex.sendError(err)
		return
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ex.debug},
	}
	var request *http.Request
	defer func(request *http.Request) {
		if request != nil {
			tr.CancelRequest(request)
		}
	}(request)

	client := &http.Client{
		Jar:       cookies,
		Transport: tr,
	}
	if ex.pi.ResponseTimeout > 0 {
		client.Timeout = time.Duration(ex.pi.ResponseTimeout) * time.Millisecond
	}

	ex.logger.Debug("%s: New HTTP Client with timeout %d msec %s", ex.getLogPrefix(), ex.pi.ResponseTimeout, ex.pi.MailServerUrl)
	stopPolling := false
	for {
		if stopPolling == false {
			request, err = ex.newRequest()
			if err != nil {
				ex.sendError(err)
				return
			}
			go ex.doRequestResponse(client, request)
			ex.logger.Debug("%s: Waiting for response", ex.getLogPrefix())
		}
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
				ex.stop() // TODO Is this the right thing to do here?
				stopPolling = true
				
			default:
				ex.sendError(fmt.Errorf("%s: Unhandled response %v", ex.getLogPrefix(), response))
				return

			}

		case cmd := <-ex.stopCh:
			switch {
			case cmd == PingerStop:
				ex.logger.Debug("%s: got stop command", ex.getLogPrefix())
				if request != nil {
					tr.CancelRequest(request)
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
			default:
				ex.logger.Error("%s: Unknown command %d", ex.getLogPrefix(), cmd)
				continue
			}
		}
	}

}

func (ex *ExchangeClient) sendError(err error) {
	_, fn, line, _ := runtime.Caller(1)
	ex.logger.Error("%s: %s/%s:%d %s", ex.getLogPrefix(), path.Base(path.Dir(fn)), path.Base(fn), line, err)
	ex.err <- err
}

func (ex *ExchangeClient) waitForError() {
	select {
	case err := <-ex.err:
		ex.lastError = err
		ex.logger.Debug("%s: Stopping goroutines", ex.getLogPrefix())
		ex.Action(PingerStop)
		return
	}
}

// LongPoll sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) LongPoll(wait *sync.WaitGroup) error {
	if ex.stats != nil {
		go ex.stats.TallyResponseTimes()
	}
	go ex.waitForError()
	ex.mutex.Lock()
	go ex.startLongPoll()
	return nil
}

// Action sends a command to the go routine.
func (ex *ExchangeClient) Action(action PingerCommand) error {
	ex.mutex.Lock()
	defer ex.mutex.Unlock()

	if ex.command == nil {
		return errors.New("Not Polling")
	}

	if ex.stats != nil {
		switch {
		case action == PingerStop:
			ex.stats.Command <- Utils.StatsStop
		default:
		}
	}
	if ex.command != nil {
		ex.command <- action
	}
	return nil
}

// Status gets the status of the go routine
func (ex *ExchangeClient) Status() (MailClientStatus, error) {
	if ex.active {
		return MailClientStatusPinging, nil
	} else {
		return MailClientStatusError, ex.lastError
	}
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(mailInfo *MailPingInformation, debug, doStats bool, logger *logging.Logger) (*ExchangeClient, error) {
	urlInfo, err := url.Parse(mailInfo.MailServerUrl)
	if err != nil {
		return nil, err
	}
	ex := &ExchangeClient{
		urlInfo:  urlInfo,
		pi:       mailInfo,
		incoming: make(chan *http.Response),
		command:  nil,
		err:      make(chan error),
		stopCh:   nil,
		debug:    debug,
		stats:    nil,
		logger:   logger,
		active:   true,
		mutex:    &sync.Mutex{},
	}
	if doStats {
		ex.stats = Utils.NewStatLogger(logger, false)
	}
	return ex, nil
}
