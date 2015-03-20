package Pinger

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	transport  *http.Transport
	request    *http.Request
	debug      bool
	logger     *Logging.Logger
	parent     *MailClientContext
	httpClient *http.Client
	cancelled  bool
	mutex      *sync.Mutex
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(parent *MailClientContext, debug bool, logger *Logging.Logger) (*ExchangeClient, error) {
	ex := &ExchangeClient{
		parent:    parent,
		debug:     debug,
		logger:    logger.Copy(),
		cancelled: false,
		mutex:     &sync.Mutex{},
	}
	ex.logger.SetCallDepth(1)
	return ex, nil
}

func (ex *ExchangeClient) getLogPrefix() (prefix string) {
	if ex.parent != nil && ex.parent.di != nil {
		prefix = ex.parent.di.getLogPrefix() + "/ActiveSync"
	} else {
		prefix = ""
	}
	return
}

func (ex *ExchangeClient) Debug(format string, args ...interface{}) {
	ex.logger.Debug(fmt.Sprintf("%s: %s", ex.getLogPrefix(), format), args...)
}

func (ex *ExchangeClient) Info(format string, args ...interface{}) {
	ex.logger.Info(fmt.Sprintf("%s: %s", ex.getLogPrefix(), format), args...)
}

func (ex *ExchangeClient) Error(format string, args ...interface{}) {
	ex.logger.Error(fmt.Sprintf("%s: %s", ex.getLogPrefix(), format), args...)
}

func (ex *ExchangeClient) Warning(format string, args ...interface{}) {
	ex.logger.Warning(fmt.Sprintf("%s: %s", ex.getLogPrefix(), format), args...)
}

func (ex *ExchangeClient) maxResponseSize() (size int) {
	if ex.parent.pi.ExpectedReply != nil {
		size = len(ex.parent.pi.ExpectedReply)
	}
	if ex.parent.pi.NoChangeReply != nil && len(ex.parent.pi.NoChangeReply) > size {
		size = len(ex.parent.pi.NoChangeReply)
	}
	return
}

var retryResponse *http.Response

func init() {
	retryResponse = &http.Response{}
}
func (ex *ExchangeClient) doRequestResponse(responseCh chan *http.Response, errCh chan error) {
	defer Utils.RecoverCrash(ex.logger)
	ex.mutex.Lock() // prevents the longpoll from cancelling the request while we're still setting it up.
	unlockMutex := true
	defer func() {
		ex.Debug("Exiting doRequestResponse")
		ex.parent.wg.Done()
		if unlockMutex {
			ex.mutex.Unlock()
		}
	}()

	var err error
	if ex == nil || ex.parent == nil || ex.parent.pi == nil {
		if ex.logger != nil {
			ex.Warning("doRequestResponse called but structures cleaned up")
		}
		return
	}
	if ex.request != nil {
		panic("Doing doRequestResponse with an active request in process!!")
	}
	requestBody := bytes.NewReader(ex.parent.pi.RequestData)
	ex.Debug("request WBXML %s", base64.StdEncoding.EncodeToString(ex.parent.pi.RequestData))
	req, err := http.NewRequest("POST", ex.parent.pi.MailServerUrl, requestBody)
	if err != nil {
		errCh <- fmt.Errorf("Failed to create request: %s", err.Error())
		return
	}
	for k, v := range ex.parent.pi.HttpHeaders {
		if k == "Accept-Encoding" {
			// ignore this. It could mess us up.
			continue
		}
		req.Header.Add(k, v)
	}
	if header := req.Header.Get("Accept"); header == "" {
		req.Header.Add("Accept", "*/*")
	}
	if header := req.Header.Get("Accept-Language"); header == "" {
		req.Header.Add("Accept-Language", "en-us")
	}
	if header := req.Header.Get("Connection"); header == "" {
		req.Header.Add("Connection", "keep-alive")
	}

	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	if DefaultPollingContext.config.Global.DumpRequests {
		requestBytes, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			ex.Error("DumpRequest error; %v", err)
		} else {
			ex.Debug("sending request:\n%s", requestBytes)
		}
	}

	if ex.parent.pi.MailServerCredentials.Username != "" && ex.parent.pi.MailServerCredentials.Password != "" {
		req.SetBasicAuth(ex.parent.pi.MailServerCredentials.Username, ex.parent.pi.MailServerCredentials.Password)
	}
	ex.request = req // save it so we can cancel it in another routine
	ex.mutex.Unlock()
	unlockMutex = false

	// Make the request and wait for response; this could take a while
	response, err := ex.httpClient.Do(ex.request)
	ex.request = nil // unsave it. We're done with it.
	if ex.cancelled == true {
		return
	}
	if err != nil {
		// if we cancel requests, we drop into this error case. We will wind up sending
		// the retryResponse, but since no one is listening, we don't care (there's no
		// memory leakage in this case
		if strings.Contains(err.Error(), "use of closed network connection") == false {
			ex.Warning("httpClient.Do failed: %s", err.Error())
		}
		responseCh <- retryResponse
		return
	}

	// read at most ex.maxResponseSize() bytes. We do this so that an attacker can't
	// flood us with an infinte amount of data. Note we also do not (and SHOULD not)
	// pay any attention to the Content-Length header. An attacker could screw us up
	// with a wrong one.
	var responseBytes []byte
	var toRead int
	if response.StatusCode != 200 {
		toRead = 1024 // read any odd responses up to 1k, so we can adequately debug-log them.
	} else {
		toRead = ex.maxResponseSize()
	}
	responseBytes = make([]byte, toRead)
	// TODO Do I need to loop here to make sure I get the number of bytes I expect?
	n, err := response.Body.Read(responseBytes)
	if err != nil {
		if err != io.EOF {
			ex.Error("Failed ro read response: %s", err.Error())
		}
		responseCh <- retryResponse
		return
	}
	if n < toRead && n != len(ex.parent.pi.ExpectedReply) && n != len(ex.parent.pi.NoChangeReply) {
		ex.Warning("Read less than expected: %d < %d", n, toRead)
	}

	// Then close the connection. We do this just for 'cleanliness'. If we forget,
	// then keepalives won't work (not that we need them).
	response.Body.Close()

	// Now put the 'body' back onto the response for later processing (because we return a response object,
	// and the LongPoll loop wants to read it and the headers.
	cached_data := ioutil.NopCloser(bytes.NewReader(responseBytes))
	response.Body = cached_data

	ex.Debug("reply WBXML %s", base64.StdEncoding.EncodeToString(responseBytes))

	if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
		headerBytes, _ := httputil.DumpResponse(response, false)
		if err != nil {
			ex.Error("Could not dump response %+v", response)
		} else {
			ex.Debug("response:\n%s%s", headerBytes, responseBytes)
		}
	}
	responseCh <- response
}

func (ex *ExchangeClient) exponentialBackoff(sleepTime int) int {
	// return sleepTime*2, or 600, whichever is less, i.e. cap the exponential backoff at 600 seconds
	if sleepTime <= 0 {
		sleepTime = 1
	}
	n := math.Min(float64(sleepTime*2), 600)
	return int(n)
}

func (ex *ExchangeClient) LongPoll(stopCh, exitCh chan int) {
	defer Utils.RecoverCrash(ex.logger)
	defer ex.parent.wg.Done()
	askedToStop := false
	defer func(prefixStr string) {
		ex.mutex.Lock()
		ex.cancelled = true
		if ex.request != nil {
			ex.Debug("Cancelling outstanding request")
			ex.transport.CancelRequest(ex.request)
		}
		ex.mutex.Unlock()
		ex.transport.CloseIdleConnections()
		if askedToStop == false {
			ex.logger.SetCallDepth(0)
			ex.logger.Debug("%s: exiting LongPoll", prefixStr)
			ex.logger.SetCallDepth(1)
			exitCh <- 1 // tell the parent we've exited.
		}
	}(ex.getLogPrefix())

	reqTimeout := ex.parent.pi.ResponseTimeout
	reqTimeout += int64(float64(reqTimeout) * 0.1) // add 10% so we don't step on the HeartbeatInterval inside the ping
	ex.transport = &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: ex.debug},
		ResponseHeaderTimeout: time.Duration(reqTimeout) * time.Millisecond,
	}

	// check for the proxy setting. Useful for mitmproxy testing
	proxy := os.Getenv("PINGER_PROXY")
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			ex.parent.sendError(err)
			return
		}
		ex.transport.Proxy = http.ProxyURL(proxyUrl)
	}

	ex.httpClient = &http.Client{
		Transport: ex.transport,
	}
	useCookieJar := false
	if useCookieJar {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			ex.parent.sendError(err)
			return
		}
		ex.httpClient.Jar = cookieJar
	}
	ex.Debug("New HTTP Client with timeout %s %s", ex.transport.ResponseHeaderTimeout, ex.parent.pi.MailServerUrl)
	sleepTime := 0
	tooFastResponse := (time.Duration(ex.parent.pi.ResponseTimeout) * time.Millisecond) / 4
	ex.Debug("TooFast timeout set to %s", tooFastResponse)
	var responseCh chan *http.Response
	var errCh chan error
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			ex.Debug("Sleeping %s before retry", s)
			time.Sleep(s)
		}
		if errCh != nil {
			close(errCh)
		}
		errCh = make(chan error)
		if responseCh != nil {
			close(responseCh)
		}
		responseCh = make(chan *http.Response)

		timeSent := time.Now()
		ex.parent.wg.Add(1)
		ex.cancelled = false
		go ex.doRequestResponse(responseCh, errCh)
		ex.Debug("Waiting for response")
		select {
		case err := <-errCh:
			ex.parent.sendError(err)
			return

		case response := <-responseCh:
			if response == retryResponse {
				ex.Debug("Retry-response from response reader.")
				sleepTime = ex.exponentialBackoff(sleepTime)
				continue
			}
			// the response body tends to be pretty short. Let's just read it all.
			responseBody, err := ioutil.ReadAll(response.Body)
			if err != nil {
				ex.parent.sendError(err)
				response.Body.Close() // attempt to close. Ignore any errors.
				return
			}
			err = response.Body.Close()
			if err != nil {
				ex.parent.sendError(err)
				return
			}
			switch {
			case response.StatusCode != 200:
				switch {
				case response.StatusCode == 401:
					// ask the client to re-register, since nothing we could do would fix this
					ex.Warning("401 response. Telling client to re-register")
					err = ex.parent.di.push(PingerNotificationRegister)
					if err != nil {
						// don't bother with this error. The real/main error is the http status. Just log it.
						ex.Error("Push failed but ignored: %s", err.Error())
					}
					return

				default:
					// just retry
					sleepTime = ex.exponentialBackoff(sleepTime)
					ex.Warning("Response Status %s. Back to polling", response.Status)
				}
			case ex.parent.pi.NoChangeReply != nil && bytes.Compare(responseBody, ex.parent.pi.NoChangeReply) == 0:
				// go back to polling
				ex.Debug("Reply matched NoChangeReply. Back to polling")
				if time.Since(timeSent) <= tooFastResponse {
					ex.Error("Response was too fast. Doing backoff. This usually indicates that the client is still connected to the exchange server.")
					sleepTime = ex.exponentialBackoff(sleepTime)
				} else {
					sleepTime = 0 // good reply. Reset any exponential backoff stuff.
				}

			case ex.parent.pi.ExpectedReply == nil || bytes.Compare(responseBody, ex.parent.pi.ExpectedReply) == 0:
				// there's new mail!
				if ex.parent.pi.ExpectedReply != nil {
					ex.Debug("Reply matched ExpectedReply")
				}
				ex.Debug("Sending push message for new mail")
				err = ex.parent.di.push(PingerNotificationNewMail) // You've got mail!
				if err != nil {
					if DefaultPollingContext.config.Global.IgnorePushFailure == false {
						ex.parent.sendError(err)
						return
					} else {
						ex.Warning("Push failed but ignored: %s", err.Error())
					}
				}
				return

			default:
				ex.Warning("Unhandled response. Just keep polling: Headers:%+v Body:%s", response.Header, base64.StdEncoding.EncodeToString(responseBody))
				sleepTime = ex.exponentialBackoff(sleepTime)
			}

		case <-ex.parent.stopAllCh: // parent will close this, at which point this will trigger.
			ex.Debug("Was told to stop (allStop). Stopping")
			askedToStop = true
			return

		case <-stopCh:
			ex.Debug("Was told to stop. Stopping")
			askedToStop = true
			return
		}
	}
}

// SelfDelete Used to zero out the structure and let garbage collection reap the empty stuff.
func (ex *ExchangeClient) Cleanup() {
	ex.parent = nil
	if ex.transport != nil {
		ex.transport.CloseIdleConnections()
	}
}
