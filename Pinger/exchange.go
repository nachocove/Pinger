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
	debug      bool
	logger     *Logging.Logger
	pi         *MailPingInformation
	wg         *sync.WaitGroup
	transport  *http.Transport
	request    *http.Request
	mutex      *sync.Mutex
	httpClient *http.Client
	cancelled  bool
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(pi *MailPingInformation, wg *sync.WaitGroup, debug bool, logger *Logging.Logger) (*ExchangeClient, error) {
	// TODO Check the request URL here. Check for http/https, and validate/sanity check the URL itself.
	// TODO check that fields we actually look at are used and fields we don't use are ignored.
	ex := &ExchangeClient{
		debug:     debug,
		logger:    logger.Copy(),
		pi:        pi,
		wg:        wg,
		mutex:     &sync.Mutex{},
		cancelled: false,
	}
	ex.logger.SetCallDepth(1)
	return ex, nil
}

func (ex *ExchangeClient) getLogPrefix() (prefix string) {
	prefix = ex.pi.getLogPrefix() + "/ActiveSync"
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
	if ex.pi.ExpectedReply != nil {
		size = len(ex.pi.ExpectedReply)
	}
	if ex.pi.NoChangeReply != nil && len(ex.pi.NoChangeReply) > size {
		size = len(ex.pi.NoChangeReply)
	}
	return
}

// This dummy response is used to indicate to the receiver of the http reply that we need to retry.
var retryResponse *http.Response

func init() {
	retryResponse = &http.Response{}
}

// doRequestResponse is used as a go-routine to do the actual send/receive (blocking) of the exchange messages.
func (ex *ExchangeClient) doRequestResponse(responseCh chan *http.Response, errCh chan error) {
	defer Utils.RecoverCrash(ex.logger)
	ex.mutex.Lock() // prevents the longpoll from cancelling the request while we're still setting it up.
	unlockMutex := true
	defer func() {
		ex.Debug("Exiting doRequestResponse")
		ex.wg.Done()
		if unlockMutex {
			ex.mutex.Unlock()
		}
	}()

	var err error
	if ex == nil || ex.pi == nil {
		if ex.logger != nil {
			ex.Warning("doRequestResponse called but structures cleaned up")
		}
		return
	}
	if ex.request != nil {
		ex.Warning("Doing doRequestResponse with an active request in process!!")
		return
	}
	requestBody := bytes.NewReader(ex.pi.RequestData)
	ex.Debug("request WBXML %s", base64.StdEncoding.EncodeToString(ex.pi.RequestData))
	req, err := http.NewRequest("POST", ex.pi.MailServerUrl, requestBody)
	if err != nil {
		errCh <- fmt.Errorf("Failed to create request: %s", err.Error())
		return
	}
	for k, v := range ex.pi.HttpHeaders {
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

	if globals.config.DumpRequests {
		requestBytes, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			ex.Error("DumpRequest error: %v", err)
		} else {
			ex.Debug("sending request:\n%s", requestBytes)
		}
	}

	if ex.pi.MailServerCredentials.Username != "" && ex.pi.MailServerCredentials.Password != "" {
		req.SetBasicAuth(ex.pi.MailServerCredentials.Username, ex.pi.MailServerCredentials.Password)
	}
	ex.request = req // save it so we can cancel it in another routine
	ex.mutex.Unlock()
	unlockMutex = false

	ex.Info("Making Connection to server")
	// Make the request and wait for response; this could take a while
	// TODO Can we and how do we validate the server certificate?
	// TODO Can we guard against bogus SSL negotiation from a hacked server?
	// TODO Perhaps we need to read and assess the Go SSL/TLS implementation
	response, err := ex.httpClient.Do(ex.request)
	ex.request = nil // unsave it. We're done with it.
	if ex.cancelled == true {
		ex.Debug("request cancelled. Exiting.")
		return
	}
	if err != nil {
		// if we cancel requests, we drop into this error case. We will wind up sending
		// the retryResponse, but since no one is listening, we don't care (there's no
		// memory leakage in this case
		// TODO Can 'err.Error()' be data from the remote endpoint? Do we need to protect against it?
		// TODO Perhaps limit the length we log here.
		if strings.Contains(err.Error(), "use of closed network connection") == false {
			ex.Warning("httpClient.Do failed: %s", err.Error())
		} else {
			ex.Debug("%s", err.Error())
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
	if len(responseBytes) != toRead {
		ex.Error("len of response is not what we expected")
		return
	}
	// TODO Do I need to loop here to make sure I get the number of bytes I expect? i.e. like unix sockets returning less than you expected.
	n, err := response.Body.Read(responseBytes)
	if err != nil {
		if err != io.EOF {
			ex.Error("Failed to read response: %s", err.Error())
		} else {
			ex.Debug("EOF from body read")
		}
		responseCh <- retryResponse
		return
	}
	if n < toRead && n != len(ex.pi.ExpectedReply) && n != len(ex.pi.NoChangeReply) {
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

	if globals.config.DumpRequests || response.StatusCode >= 500 {
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

func (ex *ExchangeClient) sendError(errCh chan error, err error) {
	logError(err, ex.logger)
	errCh <- err
}

func (ex *ExchangeClient) cancel() {
	ex.mutex.Lock()
	ex.cancelled = true
	if ex.request != nil {
		ex.Info("Cancelling outstanding request")
		ex.transport.CancelRequest(ex.request)
	}
	ex.mutex.Unlock()
	ex.transport.CloseIdleConnections()
}

// LongPoll is called by the FSM loop to do the actual work.
//
// stopPollCh - used by the parent loop to tell us to stop. This is used when a defer comes in, and
//    we just need the polling itself to stop.
//
// stopAllCh - this is a sort of broadcast mechanism used by the parent loop to indicate to all its children
//    that they can exit. This is simpler than keeping track of all the per-child stop channels.
//
// errCh - used to pass back results to the caller. The results are not just errors, but can be some
//    dummy errors like LongPollReRegister (tell the device to register), and LongPollNewMail (tell
//    the device there's new mail). This reduces the number of channels we'd use, i.e. instead of using
//    the dummy errors, we'd need a resultsChannel or some kind.
//
func (ex *ExchangeClient) LongPoll(stopPollCh, stopAllCh chan int, errCh chan error) {
	defer Utils.RecoverCrash(ex.logger) // catch all panic. RecoverCrash logs information needed for debugging.

	ex.wg.Add(1)
	defer ex.wg.Done()

	defer ex.cancel()

	var err error
	reqTimeout := ex.pi.ResponseTimeout
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
			ex.sendError(errCh, err)
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
			ex.sendError(errCh, err)
			return
		}
		ex.httpClient.Jar = cookieJar
	}
	ex.Debug("New HTTP Client with timeout %s %s", ex.transport.ResponseHeaderTimeout, ex.pi.MailServerUrl)
	sleepTime := 0
	tooFastResponse := (time.Duration(ex.pi.ResponseTimeout) * time.Millisecond) / 4
	ex.Debug("TooFast timeout set to %s", tooFastResponse)
	var responseCh chan *http.Response
	var responseErrCh chan error
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			ex.Debug("Sleeping %s before retry", s)
			time.Sleep(s)
		}
		if responseErrCh != nil {
			close(responseErrCh)
		}
		responseErrCh = make(chan error)
		if responseCh != nil {
			close(responseCh)
		}
		responseCh = make(chan *http.Response)

		timeSent := time.Now()
		ex.wg.Add(1)
		ex.cancelled = false
		go ex.doRequestResponse(responseCh, responseErrCh)
		ex.Debug("Waiting for response")
		select {
		case err = <-responseErrCh:
			ex.sendError(errCh, err)
			return

		case response := <-responseCh:
			if response == retryResponse {
				ex.Debug("Retry-response from response reader.")
				sleepTime = ex.exponentialBackoff(sleepTime)
				continue
			}
			// the response body tends to be pretty short (and we've capped it anyway). Let's just read it all.
			responseBody, err := ioutil.ReadAll(response.Body)
			if err != nil {
				response.Body.Close() // attempt to close. Ignore any errors.
				ex.sendError(errCh, err)
				return
			}
			err = response.Body.Close()
			if err != nil {
				ex.sendError(errCh, err)
				return
			}
			switch {
			case response.StatusCode != 200:
				switch {
				case response.StatusCode == 401:
					// ask the client to re-register, since nothing we could do would fix this
					ex.Warning("401 response. Telling client to re-register")
					errCh <- LongPollReRegister
					return

				default:
					// just retry
					sleepTime = ex.exponentialBackoff(sleepTime)
					ex.Warning("Response Status %s. Back to polling", response.Status)
				}

			case ex.pi.NoChangeReply != nil && bytes.Compare(responseBody, ex.pi.NoChangeReply) == 0:
				// go back to polling
				if time.Since(timeSent) <= tooFastResponse {
					ex.Info("NoChangeReply was too fast. Doing backoff. This usually indicates that the client is still connected to the exchange server.")
					sleepTime = ex.exponentialBackoff(sleepTime)
				} else {
					ex.Info("NoChangeReply after %s. Back to polling", time.Since(timeSent))
					sleepTime = 0 // good reply. Reset any exponential backoff stuff.
				}

			case ex.pi.ExpectedReply == nil || bytes.Compare(responseBody, ex.pi.ExpectedReply) == 0:
				// there's new mail!
				if ex.pi.ExpectedReply != nil {
					ex.Info("Reply matched ExpectedReply")
				}
				ex.Debug("Got mail. Setting LongPollNewMail")
				errCh <- LongPollNewMail
				return

			default:
				ex.Warning("Unhandled response. Just keep polling: Headers:%+v Body:%s", response.Header, base64.StdEncoding.EncodeToString(responseBody))
				sleepTime = ex.exponentialBackoff(sleepTime)
			}

		case <-stopPollCh: // parent will close this, at which point this will trigger.
			ex.Debug("Was told to stop. Stopping")
			return

		case <-stopAllCh: // parent will close this, at which point this will trigger.
			ex.Debug("Was told to stop (allStop). Stopping")
			return
		}
	}
}

// SelfDelete Used to zero out the structure and let garbage collection reap the empty stuff.
func (ex *ExchangeClient) Cleanup() {
	ex.pi.cleanup()
	ex.pi = nil
	if ex.transport != nil {
		ex.transport.CloseIdleConnections()
	}
}
