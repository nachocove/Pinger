package Pinger

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IMAP Commands
const (
	IMAP_EXISTS       string = "EXISTS"
	IMAP_EXPUNGE      string = "EXPUNGE"
	IMAP_EXAMINE      string = "EXAMINE"
	IMAP_IDLE         string = "IDLE"
	IMAP_DONE         string = "DONE"
	IMAP_NOOP         string = "NOOP"
	IMAP_UIDNEXT      string = "[UIDNEXT"
	IMAP_STATUS       string = "STATUS"
	IMAP_STATUS_QUERY string = "(MESSAGES UIDNEXT)"
)

// Timeout values for the Dial functions.
const (
	netTimeout    = 30 * time.Second // Time to establish a TCP connection
	clientTimeout = 60 * time.Second // Time to receive greeting and capabilities
)

type cmdTag struct {
	id  []byte
	seq uint64
}

type IMAPClient struct {
	debug      bool
	logger     *Logging.Logger
	pi         *MailPingInformation
	wg         *sync.WaitGroup
	mutex      *sync.Mutex
	cancelled  bool
	url        *url.URL
	tlsConfig  *tls.Config
	tlsConn    *tls.Conn
	scanner    *bufio.Scanner
	tag        *cmdTag
	isPolling  bool
	hasNewEmail bool
}
var prng *rand.Rand

func init() {
	prng = rand.New(&prngSource{src: rand.NewSource(time.Now().UnixNano())})
}

func (imap *IMAPClient) getLogPrefix() (prefix string) {
	prefix = imap.pi.getLogPrefix() + ":" + imap.tag.String() + "/IMAP"
	return
}

func (imap *IMAPClient) Debug(format string, args ...interface{}) {
	imap.logger.Debug(fmt.Sprintf("%s: %s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Info(format string, args ...interface{}) {
	imap.logger.Info(fmt.Sprintf("%s: %s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Error(format string, args ...interface{}) {
	imap.logger.Error(fmt.Sprintf("%s: %s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Warning(format string, args ...interface{}) {
	imap.logger.Warning(fmt.Sprintf("%s: %s", imap.getLogPrefix(), format), args...)
}

func NewIMAPClient(pi *MailPingInformation, wg *sync.WaitGroup, debug bool, logger *Logging.Logger) (*IMAPClient, error) {
	imap := IMAPClient{
		debug:     debug,
		logger:    logger.Copy(),
		pi:        pi,
		wg:        wg,
		mutex:     &sync.Mutex{},
		cancelled: false,
		tag:       genNewCmdTag(0),
	}
	imap.logger.SetCallDepth(1)
	imap.Debug("Created new IMAP Client %s", imap.getLogPrefix())
	return &imap, nil
}

func (imap *IMAPClient) sendError(errCh chan error, err error) {
	logError(err, imap.logger)
	errCh <- err
}

type prngSource struct {
	mu  sync.Mutex
	src rand.Source
}

func (r *prngSource) Int63() (n int64) {
	r.mu.Lock()
	n = r.src.Int63()
	r.mu.Unlock()
	return
}

func (r *prngSource) Seed(seed int64) {
	r.mu.Lock()
	r.src.Seed(seed)
	r.mu.Unlock()
}

func genNewCmdTag(n int) *cmdTag {
	if n < 1 || 26 < n {
		n = 5
	}
	id := make([]byte, n, n+20)
	for i, v := range prng.Perm(26)[:n] {
		id[i] = 'A' + byte(v)
	}
	return &cmdTag{id, 0}
}

func (t *cmdTag) Next() string {
	t.seq++
	return string(strconv.AppendUint(t.id, t.seq, 10))
}

func (t *cmdTag) String() string {
	return fmt.Sprintf("%s%d", t.id, t.seq)
}

func (imap *IMAPClient) ScanIMAPTerminator(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	lines := bytes.Split(data, imap.pi.CommandAcknowledgement)
	if len(lines) > 0 {
		return len(lines[0]) + len(imap.pi.CommandAcknowledgement), lines[0], nil
	}
	// Request more data.
	return 0, nil, nil
}

func (imap *IMAPClient) setupScanner() {
	if len(imap.pi.CommandTerminator) <= 0 {
		imap.pi.CommandTerminator = []byte("\r\n")
	}
	if len(imap.pi.CommandAcknowledgement) <= 0 {
		imap.pi.CommandAcknowledgement = []byte("\r\n")
	}
	imap.scanner = bufio.NewScanner(imap.tlsConn)
	imap.scanner.Split(imap.ScanIMAPTerminator)
}

func (imap *IMAPClient) handleGreeting() error {
	response, err := imap.getServerResponse(0)
	if err == nil {
		imap.Info("Connected to %s (Tag=%s)", imap.url.Host, imap.tag.id)
		if response[0:4] != "* OK" {
			err = fmt.Errorf("Did not get proper response from imap server: %s", response)
			return err
		}
		imap.Info("Greetings from server: %s", response)
	}
	return err
}

func (imap *IMAPClient) doImapAuth() (authSucess bool, err error) {
	imap.Debug("Authenticating with authblob %s", imap.pi.IMAPAuthenticationBlob)
	decodedBlob, err := base64.StdEncoding.DecodeString(imap.pi.IMAPAuthenticationBlob)
	if err != nil {
		imap.Error("Error decoding AuthBlob")
		return err
	}
	imap.Debug("authblob %s", decodedBlob)

	responses, err := imap.doIMAPCommand(fmt.Sprintf("%s %s", imap.tag.Next(), decodedBlob), 0)
	if err != nil {
		return false, err
	}
	response := responses[len(responses)-1]
	tokens := strings.Split(response, " ")
	if tokens[1] != "OK" {
		err = fmt.Errorf("%s", response)
		return false, err
	}
	return true, nil
}
func (imap *IMAPClient) parseEXAMINEResponse(response string) (value int, token string) {
	tokens := strings.Split(response, " ")
	valueToken := ""
	if tokens[0] == "*" && tokens[2] == IMAP_EXISTS {
		valueToken = tokens[1]
	} else if tokens[0] == "*" && tokens[2] == IMAP_UIDNEXT {
		valueToken = tokens[3][:len(tokens[3])-1]
	}
	if valueToken != "" {
		value, err := strconv.Atoi(valueToken)
		if err != nil {
			imap.Warning("Cannot parse value from %s", response)
		} else {
			return value, tokens[2]
		}
	}
	return 0, ""
}

//* STATUS "INBOX" (MESSAGES 18 UIDNEXT 41)
func (imap *IMAPClient) parseSTATUSResponse(response string) (int, int) {
	re := regexp.MustCompile(".*(MESSAGES (?P<messageCount>[0-9]+) UIDNEXT (?P<UIDNext>[0-9]+))")
	r2 := re.FindStringSubmatch(response)
	if len(r2) == 0 {
		return 0, 0
	}
	messageCountStr := r2[2]
	UIDNextStr := r2[3]
	messageCount, err := strconv.Atoi(messageCountStr)
	if err != nil {
		imap.Warning("Cannot parse value from %s", messageCountStr)
		messageCount = 0
	}
	UIDNext, err := strconv.Atoi(UIDNextStr)
	if err != nil {
		imap.Warning("Cannot parse value from %s", UIDNextStr)
		UIDNext = 0
	}
	return messageCount, UIDNext
}

func (imap *IMAPClient) parseIDLEResponse(response string) (value int, token string) {
	tokens := strings.Split(response, " ")
	if tokens[0] == "*" && (tokens[2] == IMAP_EXISTS || tokens[2] == IMAP_EXPUNGE) {
		value, err := strconv.Atoi(tokens[1])
		if err != nil {
			imap.Warning("Cannot parse value from %s", response)
		} else {
			return value, tokens[2]
		}
	}
	return 0, ""
}

func (imap *IMAPClient) doExamine() error {
	imap.Debug("Folder %s", imap.pi.IMAPFolderName)
	command := fmt.Sprintf("%s %s %s", imap.tag.Next(), IMAP_EXAMINE, imap.pi.IMAPFolderName)
	responses, err := imap.doIMAPCommand(command, 0)
	if err != nil {
		return err
	}
	response := responses[len(responses)-1]
	tokens := strings.Split(response, " ")
	if tokens[1] != "OK" {
		err = fmt.Errorf("Error running command %s: %s", command, response)
		return err
	}
	return nil
}

func (imap *IMAPClient) sendIMAPCommand(command string) error {
	imap.Debug("Sending IMAP Command to server:[%s]", command)
	if len(command) > 0 {
		_, err := imap.tlsConn.Write([]byte(command))
		if err != nil {
			return err
		}
		_, err = imap.tlsConn.Write(imap.pi.CommandTerminator)
		if err != nil {
			return err
		}
	}
	return nil
}

func (imap *IMAPClient) doIMAPCommand(command string, waitTime int64) ([]string, error) {
	err := imap.sendIMAPCommand(command)
	if err != nil {
		return nil, err
	}
	responses, err := imap.getServerResponses(command, waitTime)
	return responses, err
}

func (imap *IMAPClient) processResponse(command string, response string) {
	commandTokens := strings.Split(command, " ")
	if len(commandTokens) > 1 {
		commandName := commandTokens[1]
		switch {
		case commandName == "IDLE":
			imap.Debug("Processing IDLE Response: %s", response)
			count, token := imap.parseIDLEResponse(response)
			if token == IMAP_EXPUNGE {
				imap.pi.IMAPEXISTSCount -= 1
				imap.Debug("%s received. Decrementing IMAPEXISTSCount to %d", IMAP_EXPUNGE, imap.pi.IMAPEXISTSCount)

			} else if token == IMAP_EXISTS && count != imap.pi.IMAPEXISTSCount {
				imap.Debug("Current EXISTS count %d is different from starting EXISTS count %d. Resetting count...", count, imap.pi.IMAPEXISTSCount)
				imap.Debug("Got new mail. Stopping IDLE..")
				imap.hasNewEmail = true
				imap.pi.IMAPEXISTSCount = count
				err := imap.sendIMAPCommand(IMAP_DONE)
				if err != nil {
					imap.Error("Error sending IMAP Command %s: %s", IMAP_DONE, err)
				}
			}
		case commandName == "EXAMINE":
			imap.Debug("Processing EXAMINE Response: %s", response)
			count, token := imap.parseEXAMINEResponse(response)
			if token == IMAP_EXISTS {
				imap.Debug("Setting PI.IMAPEXISTSCount to %d", count)
				imap.pi.IMAPEXISTSCount = count
			} else if token == IMAP_UIDNEXT {
				imap.Debug("Setting PI.IMAPUIDNEXT to %d", count)
				imap.pi.IMAPUIDNEXT = count
			}
		case commandName == "STATUS":
			imap.Debug("Processing STATUS Response: %s", response)
			_, UIDNext := imap.parseSTATUSResponse(response)
			if UIDNext != 0 {
				if UIDNext != imap.pi.IMAPUIDNEXT {
					imap.Debug("UIDNext %d is different from starting UIDNext %d. Resetting UIDNext", UIDNext, imap.pi.IMAPUIDNEXT)
					imap.Debug("Got new mail.")
					imap.hasNewEmail = true
					imap.pi.IMAPUIDNEXT = UIDNext
				} else {
					imap.Debug("STATUS UIDNext %d is the same as starting UIDNext %d", UIDNext, imap.pi.IMAPUIDNEXT)
				}
			}
		}
	}
}

func (imap *IMAPClient) isFinalResponse(command string, response string) bool {
	tokens := strings.Split(command, " ")
	if len(tokens) > 0 {
		token := tokens[0]
		if token == response[0:len(token)] {
			return true
		}
	}
	return false
}

func (imap *IMAPClient) getServerResponses(command string, waitTime int64) ([]string, error) {
	completed := false
	responses := make([]string, 0)

	for completed == false {
		response, err := imap.getServerResponse(waitTime)
		if err != nil {
			return responses, err
		} else {
			imap.Debug(response)
			responses = append(responses, response)
			imap.processResponse(command, response)
			if imap.isFinalResponse(command, response) {
				for i, r := range responses {
					imap.Debug("%d: %s", i, r)
				}
				break
			}
		}
	}
	return responses, nil
}

func (imap *IMAPClient) getServerResponse(waitTime int64) (string, error) {
	if waitTime > 0 {
		waitUntil := time.Now().Add(time.Duration(waitTime) * time.Millisecond)
		imap.tlsConn.SetReadDeadline(waitUntil)
		defer imap.tlsConn.SetReadDeadline(time.Time{})
	}
	if ok := imap.scanner.Scan(); ok == false {
		err := imap.scanner.Err()
		if err == nil {
			err = fmt.Errorf("Could not scan connection")
		}
		return "", err
	}
	response := imap.scanner.Text()
	return response, nil
}

func (imap *IMAPClient) doRequestResponse(request string, responseCh chan []string, errCh chan error) {
	defer Utils.RecoverCrash(imap.logger)
	imap.mutex.Lock() // prevents the longpoll from cancelling the request while we're still setting it up.
	unlockMutex := true
	defer func() {
		imap.Debug("Exiting doRequestResponse")
		imap.wg.Done()
		if unlockMutex {
			imap.mutex.Unlock()
		}
	}()

	var err error
	if imap == nil || imap.pi == nil {
		if imap.logger != nil {
			imap.Warning("doRequestResponse called but structures cleaned up")
		}
		return
	}
	if imap.isPolling {
		panic("Doing doRequestResponse with an active poll in process!!")
	}
	imap.isPolling = true
	imap.mutex.Unlock()
	unlockMutex = false
	responses, err := imap.doIMAPCommand(request, 0)
	imap.isPolling = false
	if imap.cancelled == true {
		imap.Debug("Request cancelled. Exiting...")
		return
	}
	if err != nil {
		errCh <- err
		return
	}
	responseCh <- responses
	return
}

func defaultPort(addr, port string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		addr = net.JoinHostPort(addr, port)
	}
	return addr
}

func (imap *IMAPClient) setupConn() error {
	imap.Debug("Setting up TLS connection...")
	if imap.tlsConn != nil {
		imap.tlsConn.Close()
	}
	if imap.url == nil {
		imapUrl, err := url.Parse(imap.pi.MailServerUrl)
		if err != nil {
			return err
		}
		imap.url = imapUrl
	}
	host, _, _ := net.SplitHostPort(imap.url.Host)
	if imap.tlsConfig == nil {
		imap.tlsConfig = &tls.Config{ServerName: host}
	}
	conn, err := net.DialTimeout("tcp", imap.url.Host, netTimeout)
	if err == nil {
		imap.tlsConn = tls.Client(conn, imap.tlsConfig)
		//if c, err = NewClient(tlsConn, host, clientTimeout); err != nil {
		if imap.tlsConn == nil {
			conn.Close()
			return fmt.Errorf("Cannot create TLS Connection")
		}
	}
	if err != nil {
		imap.Debug("err %s", err)

		return err
	}
	imap.setupScanner()

	err = imap.handleGreeting()
	if err != nil {
		return err
	}
	return nil
}

func (imap *IMAPClient) LongPoll(stopPollCh, stopAllCh chan int, errCh chan error) {
	imap.Debug("Starting LongPoll")
	imap.wg.Add(1)
	defer imap.wg.Done()
	// TODO: why do we need this?
	//defer imap.cancel()
	defer Utils.RecoverCrash(imap.logger)
	defer func() {
		imap.Debug("Stopping...")
		if imap.tlsConn != nil {
			imap.tlsConn.Close()
			imap.tlsConn = nil
		}
	}()
	sleepTime := 0
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			imap.Debug("Sleeping %s before retry", s)
			time.Sleep(s)
		}
		sleepTime = 5
		if imap.tlsConn == nil {
			err := imap.setupConn()
			if err != nil {
				imap.Error("Connection setup error: %v", err)
				return
			}
			authSuccess, err := imap.doImapAuth()
			if err != nil {
				imap.Error("Authentication error: %v", err)
				return
			}
			if !authSuccess {
				imap.Warning("Authentication failed. Telling client to re-register")
				errCh <- LongPollReRegister
			}
			err = imap.doExamine()
			if err != nil {
				imap.Error("%v", err)
				return
			}
		}
		reqTimeout := imap.pi.ResponseTimeout
		reqTimeout += int64(float64(reqTimeout) * 0.1) // add 10% so we don't step on the HeartbeatInterval inside the ping
		requestTimer := time.NewTimer(time.Duration(reqTimeout) * time.Millisecond)
		responseCh := make(chan []string)
		command := IMAP_NOOP
		if imap.pi.IMAPSupportsIdle {
			imap.Debug("supporting idle")
			command = fmt.Sprintf("%s %s", imap.tag.Next(), IMAP_IDLE)
		} else {
			command = fmt.Sprintf("%s %s %s %s", imap.tag.Next(), IMAP_STATUS, imap.pi.IMAPFolderName, IMAP_STATUS_QUERY)
		}
		imap.Debug("command %s", command)
		imap.wg.Add(1)
		go imap.doRequestResponse(command, responseCh, errCh)
		select {
		case <-requestTimer.C:
			// request timed out. Start over.
			imap.Debug("Request timed out restarting...")
			requestTimer.Stop()
			imap.isPolling = false
			if imap.tlsConn != nil {
				imap.tlsConn.Close()
			}
			imap.tlsConn = nil

		case err := <-errCh:
			imap.sendError(errCh, err)
			return

		case <-responseCh:
			requestTimer.Stop()
			if imap.hasNewEmail {
				imap.Debug("Got mail. Setting LongPollNewMail")
				imap.hasNewEmail = false
				errCh <- LongPollNewMail
				return
			}

		case <-stopPollCh: // parent will close this, at which point this will trigger.
			imap.Debug("Was told to stop. Stopping")
			return

		case <-stopAllCh: // parent will close this, at which point this will trigger.
			imap.Debug("Was told to stop (allStop). Stopping")
			return
		}
	}
}

func (imap *IMAPClient) cancel() {
	imap.Info("Cancel called")
	imap.mutex.Lock()
	imap.cancelled = true
	if imap.isPolling {
		imap.Info("Cancelling outstanding request")
		//TODO cancel IDLE poll
	}
	imap.isPolling = false
	if imap.tlsConn != nil {
		imap.tlsConn.Close()
		imap.tlsConn = nil
	}
	imap.mutex.Unlock()
}

func (imap *IMAPClient) Cleanup() {
	imap.pi.cleanup()
	imap.pi = nil
	imap.isPolling = false
	if imap.tlsConn != nil {
		imap.tlsConn.Close()
		imap.tlsConn = nil
	}
}
