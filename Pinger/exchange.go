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
	"net/url"
	"os"
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
	parent     *MailClientContext
	httpClient *http.Client
}

// NewExchangeClient set up a new exchange client
func NewExchangeClient(parent *MailClientContext, debug bool, logger *logging.Logger) (*ExchangeClient, error) {
	ex := &ExchangeClient{
		parent:   parent,
		incoming: make(chan *http.Response),
		debug:    debug,
		logger:   logger,
	}
	return ex, nil
}

func (ex *ExchangeClient) getLogPrefix() (prefix string) {
	if ex.parent != nil && ex.parent.di != nil {
		prefix = "ActiveSync:" + ex.parent.di.getLogPrefix()
	}
	return
}

func (ex *ExchangeClient) doRequestResponse(errCh chan error) {
	var err error
	requestBody := bytes.NewReader(ex.parent.pi.HttpRequestData)
	ex.logger.Debug("%s: request WBXML %s", ex.getLogPrefix(), base64.StdEncoding.EncodeToString(ex.parent.pi.HttpRequestData))
	req, err := http.NewRequest("POST", ex.parent.pi.MailServerUrl, requestBody)
	if err != nil {
		errCh <- err
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
			ex.logger.Error("%s: DumpRequest error; %v", ex.getLogPrefix(), err)
		} else {
			ex.logger.Debug("%s: sending request:\n%s", ex.getLogPrefix(), requestBytes)
		}
	}

	if ex.parent.pi.MailServerCredentials.Username != "" && ex.parent.pi.MailServerCredentials.Password != "" {
		req.SetBasicAuth(ex.parent.pi.MailServerCredentials.Username, ex.parent.pi.MailServerCredentials.Password)
		if err != nil {
			errCh <- err
			return
		}
	}
	ex.request = req // save it so we can cancel it in another routine

	// Make the request and wait for response; this could take a while
	response, err := ex.httpClient.Do(ex.request)
	if err != nil {
		errCh <- err
		return
	}
	ex.request = nil // unsave it. We're done with it.

	// read all data and close the connection. We do this just for 'cleanliness'. If we forget,
	// then keepalives won't work (not that we need them).
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		errCh <- err
		return
	}
	cached_data := ioutil.NopCloser(bytes.NewReader(responseBytes))
	response.Body.Close()
	response.Body = cached_data
	ex.logger.Debug("%s: reply WBXML %s", ex.getLogPrefix(), base64.StdEncoding.EncodeToString(responseBytes))

	if DefaultPollingContext.config.Global.DumpRequests || response.StatusCode >= 500 {
		headerBytes, _ := httputil.DumpResponse(response, false)
		if err != nil {
			ex.logger.Error("%s: Could not dump response %+v", ex.getLogPrefix(), response)
		} else {
			ex.logger.Debug("%s: response:\n%s%s", ex.getLogPrefix(), headerBytes, responseBytes)
		}
	}
	ex.incoming <- response
}

func (ex *ExchangeClient) LongPoll(stopCh, exitCh chan int) {
	defer recoverCrash(ex.logger)
	askedToStop := false
	defer func() {
		if ex.request != nil {
			ex.transport.CancelRequest(ex.request)
		}
		ex.transport.CloseIdleConnections()
		if askedToStop == false {
			ex.logger.Debug("%s: Stopping", ex.getLogPrefix())
			exitCh <- 1 // tell the parent we've exited.
		}
	}()

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
	ex.logger.Debug("%s: New HTTP Client with timeout %s %s", ex.getLogPrefix(), ex.transport.ResponseHeaderTimeout, ex.parent.pi.MailServerUrl)
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
					sleepTime = 30 // wait 30 seconds. No need to rush it, since it may not work anyway
					ex.logger.Warning("%s: Response Status %s. Back to polling", ex.getLogPrefix(), response.Status)
				}
			case ex.parent.pi.HttpNoChangeReply != nil && bytes.Compare(responseBody, ex.parent.pi.HttpNoChangeReply) == 0:
				// go back to polling
				ex.logger.Debug("%s: Reply matched HttpNoChangeReply. Back to polling", ex.getLogPrefix())

			case ex.parent.pi.HttpExpectedReply == nil || bytes.Compare(responseBody, ex.parent.pi.HttpExpectedReply) == 0:
				// there's new mail!
				ex.logger.Debug("%s: Sending push message for new mail", ex.getLogPrefix())
				err = ex.parent.di.push(PingerNotificationNewMail) // You've got mail!
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

		case <-ex.parent.stopAllCh: // parent will close this, at which point this will trigger.
			return

		case <-stopCh:
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
