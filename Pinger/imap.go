package Pinger

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"net/url"
	"time"
)

type IMAPClient struct {
	debug     bool
	logger    *Logging.Logger
	parent    *MailClientContext
	url       *url.URL
	tlsConfig *tls.Config
	conn      *tls.Conn
	scanner   *bufio.Scanner
}

func (imap *IMAPClient) getLogPrefix() (prefix string) {
	if imap.parent != nil && imap.parent.di != nil {
		prefix = imap.parent.di.getLogPrefix() + "/IMAP"
	}
	return
}

func NewIMAPClient(parent *MailClientContext, debug bool, logger *Logging.Logger) (*IMAPClient, error) {
	imap := IMAPClient{
		debug:  debug,
		logger: logger,
		parent: parent,
	}
	return &imap, nil
}

func (imap *IMAPClient) ScanIMAPTerminator(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	lines := bytes.Split(data, imap.parent.pi.CommandAcknowledgement)
	if len(lines) > 0 {
		return len(lines[0]) + len(imap.parent.pi.CommandAcknowledgement), lines[0], nil
	}
	// Request more data.
	return 0, nil, nil
}

func (imap *IMAPClient) setupScanner() {
	if len(imap.parent.pi.CommandTerminator) <= 0 {
		imap.parent.pi.CommandTerminator = []byte("\n")
	}
	if len(imap.parent.pi.CommandAcknowledgement) <= 0 {
		imap.parent.pi.CommandAcknowledgement = []byte("\n")
	}
	imap.scanner = bufio.NewScanner(imap.conn)
	imap.scanner.Split(imap.ScanIMAPTerminator)
}

func (imap *IMAPClient) doImapAuth() error {
	response, err := imap.doIMAPCommand([]byte(fmt.Sprintf("%s AUTHENTICATE PLAIN", imap.parent.pi.ClientContext)), 0)
	if err != nil {
		return err
	}
	if bytes.Compare(response, []byte("+")) != 0 {
		err = fmt.Errorf("Did not get proper response from imap server: %s", response)
		return err
	}

	userPassBytes := []byte(fmt.Sprintf("\000%s\000%s",
		imap.parent.pi.MailServerCredentials.Username,
		imap.parent.pi.MailServerCredentials.Password))
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(userPassBytes)))
	base64.StdEncoding.Encode(buf, userPassBytes)

	response, err = imap.doIMAPCommand(buf, 0)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(response, []byte(fmt.Sprintf("%s OK AUTHENTICATE", imap.parent.pi.ClientContext))) == false {
		return fmt.Errorf("Auth failed: %s", response)
	}
	return nil
}

func (imap *IMAPClient) doIMAPCommand(command []byte, waitTime int64) ([]byte, error) {
	_, err := imap.conn.Write(command)
	if err != nil {
		return nil, err
	}
	_, err = imap.conn.Write(imap.parent.pi.CommandTerminator)
	if err != nil {
		return nil, err
	}
	if waitTime > 0 {
		waitUntil := time.Now().Add(time.Duration(waitTime) * time.Millisecond)
		imap.conn.SetReadDeadline(waitUntil)
		defer imap.conn.SetReadDeadline(time.Time{})
	}
	if ok := imap.scanner.Scan(); ok == false {
		err := imap.scanner.Err()
		if err == nil {
			err = fmt.Errorf("Could not scan connection")
		}
		return nil, err
	}
	response := imap.scanner.Text()
	if response != "+" {
		err = fmt.Errorf("Did not get proper response from imap server: %s", response)
		return nil, err
	}
	return []byte(response), nil
}

func (imap *IMAPClient) doRequestResponse(request []byte, responseCh chan []byte, errCh chan error) {
	response, err := imap.doIMAPCommand(request, 0)
	if err != nil {
		errCh <- err
		return
	}
	responseCh <- response
	return
}

func (imap *IMAPClient) setupConn() error {
	if imap.conn != nil {
		imap.conn.Close()
	}
	if imap.url == nil {
		imapUrl, err := url.Parse(imap.parent.pi.MailServerUrl)
		if err != nil {
			return err
		}
		imap.url = imapUrl
	}
	if imap.tlsConfig == nil {
		imap.tlsConfig = &tls.Config{}
	}
	conn, err := tls.Dial("tcp", imap.url.Host, imap.tlsConfig)
	if err != nil {
		return err
	}
	imap.conn = conn
	imap.setupScanner()

	err = imap.doImapAuth()
	if err != nil {
		return err
	}
	return nil
}
func (imap *IMAPClient) LongPoll(stopCh, exitCh chan int) {
	defer Utils.RecoverCrash(imap.logger)
	askedToStop := false
	defer func() {
		if imap.conn != nil {
			imap.conn.Close()
		}
		if askedToStop == false {
			imap.logger.Debug("%s: Stopping", imap.getLogPrefix())
			exitCh <- 1 // tell the parent we've exited.
		}
	}()

	sleepTime := 0
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			imap.logger.Debug("%s: Sleeping %s before retry", imap.getLogPrefix(), s)
			time.Sleep(s)
		}
		sleepTime = 1 // default sleeptime on retry. Error cases can override it.
		if imap.conn == nil {
			err := imap.setupConn()
			if err != nil {
				imap.parent.sendError(err)
				return
			}
		}

		reqTimeout := imap.parent.pi.ResponseTimeout
		reqTimeout += int64(float64(reqTimeout) * 0.1) // add 10% so we don't step on the HeartbeatInterval inside the ping
		requestTimer := time.NewTimer(time.Duration(reqTimeout) * time.Millisecond)
		errCh := make(chan error)
		responseCh := make(chan []byte)
		go imap.doRequestResponse(imap.parent.pi.RequestData, responseCh, errCh)
		select {
		case <-requestTimer.C:
			// request timed out. Start over.
			requestTimer.Stop()
			imap.conn.Close()
			err := imap.setupConn()
			if err != nil {
				imap.parent.sendError(err)
				return
			}

		case err := <-errCh:
			imap.parent.sendError(err)
			return

		case response := <-responseCh:
			requestTimer.Stop()
			switch {
			case imap.parent.pi.NoChangeReply != nil && bytes.Compare(response, imap.parent.pi.NoChangeReply) == 0:
				// go back to polling

			case imap.parent.pi.ExpectedReply == nil || bytes.Compare(response, imap.parent.pi.ExpectedReply) == 0:
				// got mail! Send push.
			}

		case <-imap.parent.stopAllCh: // parent will close this, at which point this will trigger.
			return

		case <-stopCh:
			askedToStop = true
			return
		}
	}
}

func (imap *IMAPClient) Cleanup() {
	imap.parent = nil
}
