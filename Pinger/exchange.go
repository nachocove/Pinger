package Pinger

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"time"

	"github.com/op/go-logging"
)

// ExchangeClient A client with type exchange.
type ExchangeClient struct {
	incoming   chan *http.Response
	transport  *http.Transport
	request    *http.Request
	debug      bool
	logger     *logging.Logger
	is_active  bool
	parent     *MailClientContext
	cookieJar  *cookiejar.Jar
	httpClient *http.Client
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(parent *MailClientContext, debug bool, logger *logging.Logger) (*ExchangeClient, error) {
	ex := &ExchangeClient{
		parent:    parent,
		incoming:  make(chan *http.Response),
		debug:     debug,
		logger:    logger,
		is_active: false,
	}
	return ex, nil
}

func (ex *ExchangeClient) getLogPrefix() string {
	if ex.parent != nil && ex.parent.di != nil {
		return ex.parent.di.getLogPrefix()
	}
	return ""
}

func (ex *ExchangeClient) doRequestResponse(errCh chan error) {
	var err error
	requestBody := bytes.NewReader(ex.parent.pi.HttpRequestData)
	req, err := http.NewRequest("POST", ex.parent.pi.MailServerUrl, requestBody)
	if err != nil {
		errCh <- err
		return
	}
	for k, v := range ex.parent.pi.HttpHeaders {
		req.Header.Add(k, v)
	}
	if header := req.Header.Get("User-Agent"); header == "" {
		req.Header.Add("User-Agent", "NachoCovePingerv0.9")
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
			ex.logger.Error("%s: DumpRequest error; %v", ex.getLogPrefix(), err)
		} else {
			ex.logger.Debug("%s: sending request:\n%s", ex.getLogPrefix(), requestBytes)
		}
	}

	req.SetBasicAuth(ex.parent.pi.MailServerCredentials.Username, ex.parent.pi.MailServerCredentials.Password)
	if err != nil {
		errCh <- err
		return
	}
	ex.request = req // save it so we can cancel it in another routine

	// Make the request and wait for response
	response, err := ex.httpClient.Do(ex.request)
	if err != nil {
		errCh <- err
		return
	}
	ex.request = nil
	if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
		headerBytes, _ := httputil.DumpResponse(response, false)
		responseBytes, _ := ioutil.ReadAll(response.Body)
		cached_data := ioutil.NopCloser(bytes.NewReader(responseBytes))
		response.Body.Close()
		response.Body = cached_data
		if err != nil {
			ex.logger.Error("%s: Could not dump response %+v", ex.getLogPrefix(), response)
		} else {
			ex.logger.Debug("%s: response:\n%s%s", ex.getLogPrefix(), headerBytes, responseBytes)
		}
	}
	ex.incoming <- response
}

func (ex *ExchangeClient) LongPoll(exitCh chan int) {
	defer recoverCrash(ex.logger)
	defer func() {
		exitCh <- 1 // tell the parent we've exited.
	}()

	ex.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: ex.debug},
	}
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		ex.parent.sendError(err)
		return
	}
	ex.cookieJar = cookieJar
	ex.httpClient = &http.Client{
		Jar:       ex.cookieJar,
		Transport: ex.transport,
	}
	if ex.parent.pi.ResponseTimeout > 0 {
		timeout := ex.parent.pi.ResponseTimeout
		timeout += int64(float64(timeout) * 0.1) // add 10% so we don't step on the HeartbeatInterval inside the ping
		ex.httpClient.Timeout = time.Duration(timeout) * time.Millisecond
	}
	ex.logger.Debug("%s: New HTTP Client with timeout %s %s", ex.getLogPrefix(), ex.httpClient.Timeout, ex.parent.pi.MailServerUrl)
	sleepTime := 0
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			ex.logger.Debug("%s: Sleeping %s before retry", ex.getLogPrefix(), s)
			time.Sleep(s)
		}
		sleepTime = 1 // default sleeptime on retry. Error cases can override it.
		errCh := make(chan error)
		go ex.doRequestResponse(errCh)
		ex.logger.Debug("%s: Waiting for response", ex.getLogPrefix())
		select {
		case err := <-errCh:
			ex.parent.sendError(err)
			return

		case response := <-ex.incoming:
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
					ex.logger.Warning("%s: 401 response. Telling client to re-register", ex.getLogPrefix())
					err = ex.parent.di.push(PingerNotificationRegister)
					if err != nil {
						// don't bother with this error. The real/main error is the http status. Just log it.
						ex.logger.Error("%s: Push failed but ignored: %s", ex.getLogPrefix(), err.Error())
					}
					return

				default:
					// just retry
					sleepTime = 10
					ex.logger.Warning("%s: Response Status %s. Back to polling", ex.getLogPrefix(), response.Status)
				}
			case ex.parent.pi.HttpNoChangeReply != nil && bytes.Compare(responseBody, ex.parent.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", ex.getLogPrefix())

			case ex.parent.pi.HttpExpectedReply == nil || bytes.Compare(responseBody, ex.parent.pi.HttpExpectedReply) == 0:
				// there's new mail!
				if bytes.HasPrefix(responseBody, []byte("HTTP/")) {
					ex.logger.Error("Response body contains the entire response!")
					ex.parent.sendError(fmt.Errorf("Response body contains the entire response"))
					return
				}
				ex.logger.Debug("%s: reply is %s", ex.getLogPrefix(), base64.StdEncoding.EncodeToString(responseBody))
				ex.logger.Debug("%s: Sending push message for new mail", ex.getLogPrefix())
				err = ex.parent.di.push(PingerNotificationNewMail)
				if err != nil {
					if DefaultPollingContext.config.Global.IgnorePushFailure == false {
						ex.parent.sendError(err)
						return
					} else {
						ex.logger.Warning("%s: Push failed but ignored: %s", ex.getLogPrefix(), err.Error())
					}
				}
				return

			default:
				ex.parent.sendError(fmt.Errorf("%s: Unhandled response %v", ex.getLogPrefix(), response))
				return

			}

		case <-ex.parent.stopCh: // parent will close this, at which point this will trigger.
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
