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
	"path"
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
	stopCh    chan int64
	
	lastError error
	waitBeforeUse int
	debug         bool
	logger        *logging.Logger
	stats         *StatLogger
	pi            *MailPingInformation
	urlInfo       *url.URL
	active        bool
	deviceInfo    *DeviceInfo
	logPrefix     string
	mutex         *sync.Mutex
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
		requestBytes, err := httputil.DumpRequest(request, false)
		if err != nil {
			ex.logger.Error("DumpRequest error; %v", err)
		} else {
			ex.logger.Debug("%s: sending request: %s", ex.getLogPrefix(), requestBytes)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		ex.sendError(err)
		return
	}
	ex.incoming <- response
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
		return err
		}
		ex.logger.Debug("%s: endpoint created %s", ex.getLogPrefix(), deviceInfo.AWSEndpointArn)
		// TODO We should send a test-ping here, so we don't find out the endpoint is unreachable later.
		// It's optional (we'll find out eventually), but this would speed it up.
	} else {
		// Validate this even if the device is marked as deviceInfo.Enabled=false, because this might
		// mark it as enabled again. Possibly...
		err = deviceInfo.validateAws()
		if err != nil {
		return err
		}
		ex.logger.Debug("%s: endpoint validated %s", ex.getLogPrefix(), deviceInfo.AWSEndpointArn)
	}
	ex.deviceInfo = deviceInfo
	return nil	
}

// This function needs refactoring to better support a clean 'defer'
func (ex *ExchangeClient) startLongPoll() {
	defer recoverCrash(ex.logger)
	ex.command = make(chan int, 10)
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
	
	ex.logger.Debug("Starting deferTimer for %d seconds", ex.pi.WaitBeforeUse)
	deferTimer := time.NewTimer(time.Duration(ex.pi.WaitBeforeUse) * time.Second)
	
	forLoop:	
	for {
		ex.logger.Debug("Top of state machine loop")
		select {
		case <-deferTimer.C:
			ex.logger.Debug("DeferTimer expired. Running.")
			ex.mutex.Lock()
			ex.logger.Debug("Mutex before run locked")
			go ex.run()  // will unlock mutex, when the stop channel is initialized. Prevents race-condition where we get a Stop/Defer just as the go routine is starting
	
		case cmd := <-ex.command:
			switch {
			case cmd == Stop:
				ex.logger.Debug("%s: got stop command", ex.getLogPrefix())
				deferTimer.Stop()
				ex.stop()
				break forLoop
				
			case cmd == Defer:
				ex.logger.Debug("reStarting deferTimer for %d seconds", ex.pi.WaitBeforeUse)
				ex.stop()
				deferTimer.Reset(time.Duration(ex.pi.WaitBeforeUse) * time.Second)
				
			default:
				ex.logger.Error("Unknown command %d", cmd)
				continue
			
			}
		}
	}
}

func (ex *ExchangeClient) stop() {
	ex.mutex.Lock()
	ex.logger.Debug("Mutex in stop() locked")
	defer ex.mutex.Unlock()
	ex.logger.Debug("stopCh is %+v", ex.stopCh)
	if ex.stopCh != nil {
		ex.stopCh<-Stop
	}
	ex.logger.Debug("Mutex in stop() unlocked")
}

func (ex *ExchangeClient) run() {
	ex.stopCh = make(chan int64, 2)
	defer func() {
		ex.stopCh = nil
	}()
	ex.mutex.Unlock()
	ex.logger.Debug("Mutex in run unlocked")
	
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

	ex.logger.Debug("%s: New HTTP Client with timeout %d %s", ex.getLogPrefix(), ex.pi.ResponseTimeout, ex.pi.MailServerUrl)
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
		ex.logger.Debug("%s: Waiting for response", ex.getLogPrefix())
		select {
		case response := <-ex.incoming:
			responseBody := make([]byte, response.ContentLength)
			_, err := response.Body.Read(responseBody)
			if err != nil && err != io.EOF {
				ex.sendError(err)
				return
			}
			err = response.Body.Close()
			if err != nil {
				ex.sendError(err)
				return
			}
			if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
				ex.logger.Debug("%s: response and body: %v %s", ex.getLogPrefix(), *response, responseBody)
			}
			switch {
			case response.StatusCode != 200:
				ex.logger.Debug("%s: Non-200 response: %d", ex.getLogPrefix(), response.StatusCode)
				ex.sendError(errors.New(fmt.Sprintf("Http %d status response", response.StatusCode)))
				return

			case ex.pi.HttpNoChangeReply != nil && bytes.Compare(responseBody, ex.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", ex.getLogPrefix())

			default:
				newMail := false
				if bytes.Compare(responseBody, ex.pi.HttpExpectedReply) == 0 {
					// there's new mail!
					ex.logger.Debug("%s: Reply matched HttpExpectedReply. Send Push", ex.getLogPrefix())
					newMail = true
				} else {
					if ex.pi.HttpNoChangeReply != nil {
						// apparently the 'no-change' above didn't match, so this must be a change
						newMail = true
					} else {
						ex.sendError(errors.New(fmt.Sprintf("%s: Unhandled response %v", ex.getLogPrefix(), response)))
						return
					}
				}
				if newMail {
					ex.logger.Debug("%s: Sending push message for new mail", ex.getLogPrefix())
					err = ex.deviceInfo.push("You've got mail!")
					if err != nil {
						ex.sendError(err)
						return
					}
				}
			}
			sleepTime = (time.Duration(ex.pi.ResponseTimeout) * time.Second) - time.Since(requestSent)

		case cmd := <- ex.stopCh:
			switch {
			case cmd == Stop:
				ex.logger.Debug("%s: got stop command", ex.getLogPrefix())
				tr.CancelRequest(request)
				resp, ok := <-ex.incoming // wait for it to cancel
				if ok {
					ex.logger.Debug("%s: lagging response %s", ex.getLogPrefix(), resp)
				}
				stopPolling = true				
			default:
				ex.logger.Error("Unknown command %d", cmd)
				continue
			}
		}
		if stopPolling == true {
			break
		}

		if sleepTime.Seconds() > 0.0 {
			ex.logger.Debug("%s: sleeping %fs before next attempt", ex.getLogPrefix(), sleepTime.Seconds())
			time.Sleep(sleepTime)
		}
	}
	
}

func (ex *ExchangeClient) sendError(err error) {
	_, fn, line, _ := runtime.Caller(1)
	ex.err <- errors.New(fmt.Sprintf("%s/%s:%d %s", path.Base(path.Dir(fn)), path.Base(fn), line, err))
}

func (ex *ExchangeClient) waitForError() {
	select {
	case err := <-ex.err:
		ex.logger.Error(err.Error())
		ex.lastError = err
		ex.logger.Debug("Stopping goroutines")
		ex.Action(Stop)
		return
	}
}

// LongPoll sets up the exchange client to listen. Most of the hard work is done via the Client.Listen()
// launches 1 goroutine for periodic checking, if confgured.
func (ex *ExchangeClient) LongPoll(wait *sync.WaitGroup) error {
	if ex.stats != nil {
		go ex.stats.tallyResponseTimes()
	}
	go ex.waitForError()
	ex.mutex.Lock()
	go ex.startLongPoll()
	return nil
}

// Action sends a command to the go routine.
func (ex *ExchangeClient) Action(action int) error {
	ex.mutex.Lock()
	if ex.command == nil {
		return errors.New("Not Polling")
	}
	
	defer ex.mutex.Unlock()
	if ex.stats != nil {
		ex.stats.Command <- action
	}
	if ex.command != nil {
		ex.logger.Debug("Sending action %d to client %s", action, ex.getLogPrefix())
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
		ex.stats = NewStatLogger(logger, false)
	}
	return ex, nil
}
