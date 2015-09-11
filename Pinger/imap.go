package Pinger

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"errors"
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
	netTimeout       = 30 * time.Second // Time to establish a TCP connection
	POLLING_INTERVAL = 30
	replyTimeout     = 300 * time.Second // Time to wait on server response
)

type cmdTag struct {
	id  []byte
	seq uint64
}

type IMAPClient struct {
	debug       bool
	logger      *Logging.Logger
	pi          *MailPingInformation
	wg          *sync.WaitGroup
	mutex       *sync.Mutex
	cancelled   bool
	url         *url.URL
	tlsConfig   *tls.Config
	tlsConn     *tls.Conn
	scanner     *bufio.Scanner
	tag         *cmdTag
	isIdling    bool
	hasNewEmail bool
}

var prng *rand.Rand
var commandTerminator []byte

func init() {
	prng = rand.New(&prngSource{src: rand.NewSource(time.Now().UnixNano())})
	commandTerminator = []byte("\r\n")
}

func (imap *IMAPClient) getLogPrefix() string {
	return imap.pi.getLogPrefix() + "|protocol=IMAP" + "|tag=" + string(imap.tag.id) + ":" + strconv.FormatUint(imap.tag.seq, 10)
}

func (imap *IMAPClient) Debug(format string, args ...interface{}) {
	imap.logger.Debug(fmt.Sprintf("%s|message=%s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Info(format string, args ...interface{}) {
	imap.logger.Info(fmt.Sprintf("%s|message=%s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Error(format string, args ...interface{}) {
	imap.logger.Error(fmt.Sprintf("%s|message=%s", imap.getLogPrefix(), format), args...)
}

func (imap *IMAPClient) Warning(format string, args ...interface{}) {
	imap.logger.Warning(fmt.Sprintf("%s|message=%s", imap.getLogPrefix(), format), args...)
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
	imap.Info("Created new IMAP Client|msgCode=IMAP_CLIENT_CREATED")
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

func genNewCmdTag(n uint) *cmdTag {
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

func (imap *IMAPClient) setupScanner() {
	imap.scanner = bufio.NewScanner(imap.tlsConn)
	imap.scanner.Split(bufio.ScanLines)
}

func (imap *IMAPClient) isContinueResponse(response string) bool {
	if len(response) > 0 && response[0] == '+' {
		return true
	} else {
		return false
	}
}

func (imap *IMAPClient) isOKResponse(response string) bool {
	tokens := strings.Split(response, " ")
	if len(tokens) >= 2 && tokens[1] == "OK" {
		return true
	} else {
		return false
	}
}

func (imap *IMAPClient) handleGreeting() error {
	imap.Debug("Handle Greeting")
	response, err := imap.getServerResponse(uint64(replyTimeout / time.Millisecond))
	if err == nil {
		imap.Info("Connected|host=%s|tag=%s", imap.url.Host, imap.tag.id)
		if imap.isOKResponse(response) {
			imap.Info("Greeting from server: %s", response)
			return nil
		} else {
			err := fmt.Errorf("Did not get proper response from imap server|err=%s", response)
			return err
		}
	}
	return err
}

func (imap *IMAPClient) doImapAuth() (authSucess bool, err error) {
	imap.Info("Authenticating with authblob")
	decodedBlob, err := base64.StdEncoding.DecodeString(imap.pi.IMAPAuthenticationBlob)
	if err != nil {
		imap.Error("Error decoding AuthBlob")
		return false, err
	}
	responses, err := imap.doIMAPCommand(fmt.Sprintf("%s %s", imap.tag.Next(), decodedBlob), uint64(replyTimeout/time.Millisecond))
	if err != nil {
		return false, err
	}
	if len(responses) > 0 {
		lastResponse := responses[len(responses)-1]
		if imap.isContinueResponse(lastResponse) { // auth failed
			imap.Debug("Authentication failed: %s", lastResponse)
			responses, err = imap.doIMAPCommand(" ", uint64(replyTimeout/time.Millisecond))
		}
		if !imap.isOKResponse(lastResponse) {
			return false, err
		}
	}
	imap.Debug("Authentication successful|msgCode=IMAP_AUTH_SUCCESS")
	return true, nil
}

func (imap *IMAPClient) parseEXAMINEResponse(response string) (value uint32, token string) {
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
			imap.Warning("Cannot parse value from response : %s", response)
		} else {
			return uint32(value), tokens[2]
		}
	}
	return 0, ""
}

//* STATUS "INBOX" (MESSAGES 18 UIDNEXT 41)
func (imap *IMAPClient) parseSTATUSResponse(response string) (uint32, uint32) {
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
	return uint32(messageCount), uint32(UIDNext)
}

func (imap *IMAPClient) parseIDLEResponse(response string) (value uint32, token string) {
	tokens := strings.Split(response, " ")
	if tokens[0] == "*" && (tokens[2] == IMAP_EXISTS || tokens[2] == IMAP_EXPUNGE) {
		value, err := strconv.Atoi(tokens[1])
		if err != nil {
			imap.Warning("Cannot parse value from %s", response)
		} else {
			return uint32(value), tokens[2]
		}
	}
	return 0, ""
}

func (imap *IMAPClient) doExamine() error {
	command := fmt.Sprintf("%s %s %s", imap.tag.Next(), IMAP_EXAMINE, imap.pi.IMAPFolderName)
	imap.Debug("IMAPFolder=%s", imap.pi.IMAPFolderName)
	_, err := imap.doIMAPCommand(command, uint64(replyTimeout/time.Millisecond))
	return err
}

func (imap *IMAPClient) sendIMAPCommand(command string) error {
	commandName := imap.getNameFromCommand(command)
	imap.Info("Sending IMAP Command to server|command=%s|msgCode=IMAP_COMMAND_SENT", commandName)
	//imap.Debug("Sending IMAP Command to server:[%s]", command)
	if commandName == "IDLE" {
		imap.Info("Setting isIdling to true.")
		imap.isIdling = true
	}
	if len(command) > 0 {
		_, err := imap.tlsConn.Write([]byte(command))
		if err != nil {
			return err
		}
		_, err = imap.tlsConn.Write(commandTerminator)
		if err != nil {
			return err
		}
	}
	return nil
}

func (imap *IMAPClient) doIMAPCommand(command string, waitTime uint64) ([]string, error) {
	commandLines := strings.Split(command, "\n")
	var allResponses []string
	var err error
	for _, commandLine := range commandLines {
		err := imap.sendIMAPCommand(commandLine)
		if err != nil {
			imap.Warning("%s", err)
			return nil, err
		}
		if imap.cancelled == true {
			imap.Info("IMAP Command. Request cancelled. Exiting|msgCode=IMAP_COMMAND_CANCELLED")
			err = fmt.Errorf("Request cancelled")
			return nil, err
		}
		responses, err := imap.getServerResponses(command, waitTime)
		if err != nil {
			return nil, err
		}
		if allResponses == nil {
			allResponses = responses
		} else {
			allResponses = append(allResponses, responses...)
		}
		if len(responses) > 0 {
			lastResponse := responses[len(responses)-1]
			if !imap.isOKResponse(lastResponse) && !imap.isContinueResponse(lastResponse) {
				err := fmt.Errorf("Did not get proper response from imap server: %s", lastResponse)
				imap.Debug("%s", err)
				return allResponses, err
			}
		} else {
			err := fmt.Errorf("Did not get any response from imap server.")
			imap.Debug("%s", err)
			return allResponses, err
		}
	}
	return allResponses, err
}

func (imap *IMAPClient) processResponse(command string, response string) {
	commandName := imap.getNameFromCommand(command)
	switch commandName {
	case "IDLE":
		imap.Debug("Processing IDLE Response: [%s]", response)
		count, token := imap.parseIDLEResponse(response)
		if token == IMAP_EXPUNGE {
			imap.pi.IMAPEXISTSCount -= 1
			imap.Info("%s received. Decrementing count|IMAPEXISTSCount=%d", IMAP_EXPUNGE, imap.pi.IMAPEXISTSCount)

		} else if token == IMAP_EXISTS && count != imap.pi.IMAPEXISTSCount {
			imap.Info("Current EXISTS count is different from starting EXISTS count."+
				"Resetting count|currentIMAPEXISTSCount=%d|startingIMAPExistsCount=%d", count, imap.pi.IMAPEXISTSCount)
			imap.Info("Got new mail. Stopping IDLE|msgCode=IMAP_NEW_MAIL")
			imap.hasNewEmail = true
			imap.pi.IMAPEXISTSCount = count
			err := imap.sendIMAPCommand(IMAP_DONE)
			if err != nil {
				imap.Warning("Error sending IMAP Command|command=%s|err=%s", IMAP_DONE, err)
			}
		}
	case "EXAMINE":
		imap.Debug("Processing EXAMINE Response: [%s]", response)
		count, token := imap.parseEXAMINEResponse(response)
		if token == IMAP_EXISTS {
			imap.Info("Saving starting EXISTS count|IMAPEXISTSCount=%d||msgCode=IMAP_STARTING_EXISTS_COUNT", count)
			imap.pi.IMAPEXISTSCount = count
		} else if token == IMAP_UIDNEXT {
			imap.Info("Setting starting IMAPUIDNEXT|IMAPUIDNEXT=%d", count)
			imap.pi.IMAPUIDNEXT = count
		}
	case "STATUS":
		imap.Debug("Processing STATUS Response: [%s]", response)
		_, UIDNext := imap.parseSTATUSResponse(response)
		if UIDNext != 0 {
			if imap.pi.IMAPUIDNEXT == 0 {
				imap.Info("Setting starting IMAPUIDNEXT|IMAPUIDNEXT=%d", UIDNext)
				imap.pi.IMAPUIDNEXT = UIDNext
			} else if UIDNext != imap.pi.IMAPUIDNEXT {
				imap.Info("Current UIDNext is different from starting UIDNext."+
					" Resetting UIDNext|currentUIDNext=%d|startingUIDNext=%d|msgCode=IMAP_RESET_UIDNEXT", UIDNext, imap.pi.IMAPUIDNEXT)
				imap.Info("Got new mail|msgCode=IMAP_NEW_MAIL")
				imap.hasNewEmail = true
				imap.pi.IMAPUIDNEXT = UIDNext
			} else {
				imap.Debug("Current UIDNext is the same as starting UIDNext|currentUIDNext=%d|startingUIDNext=%d", UIDNext, imap.pi.IMAPUIDNEXT)
			}
		}
	}
}

func (imap *IMAPClient) isFinalResponse(command string, response string) bool {
	tokens := strings.Split(command, " ")
	if len(response) >= 2 && response[0:2] == "+ " && imap.getNameFromCommand(command) != "IDLE" {
		return true
	} else if len(tokens) > 0 {
		token := tokens[0]
		if len(response) >= len(token) && token == response[0:len(token)] {
			return true
		}
	}
	return false
}

func (imap *IMAPClient) getNameFromCommand(command string) string {
	commandTokens := strings.Split(command, " ")
	if len(commandTokens) > 1 {
		return commandTokens[1]
	}
	return ""
}

func (imap *IMAPClient) getServerResponses(command string, waitTime uint64) ([]string, error) {
	completed := false
	responses := make([]string, 0)
	imap.Debug("Getting Server Responses")
	for completed == false {
		if imap.getNameFromCommand(command) == "IDLE" {
			waitTime = 0
			imap.Debug("IDLE Command|timeout=%d", waitTime)
		}
		response, err := imap.getServerResponse(waitTime)
		if err != nil {
			imap.Debug("Returning err %s", err)
			return responses, err
		} else {
			if imap.getNameFromCommand(command) == "AUTHENTICATE" {
				imap.Debug("<%s command response redacted>", imap.getNameFromCommand(command))
			} else {
				imap.Debug("IMAP Server Response is %s", response)
			}

			responses = append(responses, response)
			imap.processResponse(command, response)
			if imap.isFinalResponse(command, response) {
				if imap.getNameFromCommand(command) == "IDLE" {
					imap.Info("Setting isIdling to false|msgCode=IMAP_STOP_IDLE")
					imap.isIdling = false
				}
				for i, r := range responses {
					if imap.getNameFromCommand(command) == "AUTHENTICATE" {
						imap.Debug("%d: <%s command response redacted>", i, imap.getNameFromCommand(command))
					} else {
						imap.Debug("%d: %s", i, r)
					}
				}
				break
			}
		}
	}
	return responses, nil
}

func (imap *IMAPClient) getServerResponse(waitTime uint64) (string, error) {
	imap.Debug("Getting server response|timeout=%d", waitTime)
	if waitTime > 0 {
		waitUntil := time.Now().Add(time.Duration(waitTime) * time.Millisecond)
		imap.tlsConn.SetReadDeadline(waitUntil)
	}
	for i := 0; ; i++ {
		ok := imap.scanner.Scan()
		if ok {
			break
		} else {
			err := imap.scanner.Err()
			if err == nil {
				return "", errors.New("EOF received")
			}
			nerr, ok := err.(net.Error)
			if ok && nerr.Timeout() {
				imap.Debug("Timeout error|err=%s", nerr)
				return "", err
			} else if ok && nerr.Temporary() {
				if i < 3 { // try three times
					imap.Info("Temporary error scanning for server response: %s. Will retry...", nerr)
					time.Sleep(time.Duration(1) * time.Second)
				} else {
					imap.Debug("Error scanning for server response: %s.", nerr)
					return "", err
				}
			} else {
				imap.Debug("Error scanning for server response: %s.", err)
				return "", err
			}
		}
	}
	response := imap.scanner.Text()
	return response, nil
}

func (imap *IMAPClient) doRequestResponse(request string, responseCh chan []string, responseErrCh chan error) {
	imap.Debug("Starting doRequestResponse")
	imap.wg.Add(1)
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
			imap.Info("doRequestResponse called but structures cleaned up")
		}
		return
	}
	if imap.tlsConn == nil {
		imap.Info("doRequestResponse called but tls connection has been cleaned up")
		return
	}
	imap.mutex.Unlock()
	unlockMutex = false
	imap.Debug("Executing IMAP Command|timeout=%d", uint64(replyTimeout/time.Millisecond))
	responses, err := imap.doIMAPCommand(request, uint64(replyTimeout/time.Millisecond))
	if imap.cancelled == true {
		imap.Info("IMAP Request cancelled. Exiting|msgCode=IMAP_REQ_CANCELLED")
		return
	}
	if err != nil {
		if imap.isIdling {
			imap.isIdling = false
		}
		imap.Info("Request/Response Error: %s", err)
		responseErrCh <- fmt.Errorf("Request/Response Error: %s", err)
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
	imap.Debug("Setting up TLS connection")
	if imap.tlsConn != nil {
		imap.tlsConn.Close()
	}
	if imap.url == nil {
		imapUrl, err := url.Parse(imap.pi.MailServerUrl)
		if err != nil {
			imap.Warning("err %s", err)
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
		if imap.tlsConn == nil {
			conn.Close()
			return fmt.Errorf("Cannot create TLS Connection")
		}
	}
	if err != nil {
		imap.Warning("err %s", err)
		return err
	}
	imap.setupScanner()

	err = imap.handleGreeting()
	if err != nil {
		imap.Warning("err %s", err)
		return err
	}
	return nil
}

func (imap *IMAPClient) LongPoll(stopPollCh, stopAllCh chan int, errCh chan error) {
	imap.Info("Starting LongPoll|msgCode=POLLING")
	if imap.isIdling {
		imap.Warning("Already idling. Returning|msgCode=IMAP_ALREADY_POLLING")
		return
	}
	imap.wg.Add(1)
	defer imap.wg.Done()
	defer Utils.RecoverCrash(imap.logger)
	defer func() {
		imap.Info("Stopping LongPoll.")
		imap.cancel()
	}()
	sleepTime := 0
	if imap.pi.IMAPSupportsIdle {
		imap.Debug("IMAP Server supports IDLE")
	} else {
		imap.Debug("IMAP Server doesn't support IDLE. Resetting IMAP UIDNEXT|IMAPUIDNEXT=0|msgCode")
	}
	imap.pi.IMAPUIDNEXT = 0
	for {
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			imap.Debug("Sleeping %s before retry", s)
			time.Sleep(s)
		}
		sleepTime = POLLING_INTERVAL
		if imap.tlsConn == nil {
			err := imap.setupConn()
			if err != nil {
				imap.Error("Connection setup error: %v", err)
				errCh <- LongPollReRegister
				return
			}
			authSuccess, err := imap.doImapAuth()
			if err != nil {
				imap.Warning("Authentication failed. Telling client to re-register|msgCode=IMAP_AUTH_FAIL_REREGISTER")
				errCh <- LongPollReRegister
				return
			}
			if !authSuccess {
				imap.Warning("Authentication failed. Telling client to re-register|msgCode=IMAP_AUTH_FAIL_REREGISTER")
				errCh <- LongPollReRegister
				return
			}
		}
		if imap.pi.IMAPSupportsIdle {
			imap.Debug("Supporting idle. Running Examine Command")
			err := imap.doExamine()
			if err != nil {
				imap.Warning("Examine failure: %v. Telling client to re-register|msgCode=IMAP_AUTH_FAIL_REREGISTER", err)
				errCh <- LongPollReRegister
				return
			}
		}
		reqTimeout := imap.pi.ResponseTimeout
		reqTimeout += uint64(float64(reqTimeout) * 0.1) // add 10% so we don't step on the HeartbeatInterval inside the ping
		requestTimer := time.NewTimer(time.Duration(reqTimeout) * time.Millisecond)
		responseCh := make(chan []string)
		responseErrCh := make(chan error)
		command := IMAP_NOOP
		if imap.pi.IMAPSupportsIdle {
			command = fmt.Sprintf("%s %s", imap.tag.Next(), IMAP_IDLE)
		} else {
			command = fmt.Sprintf("%s %s %s %s", imap.tag.Next(), IMAP_STATUS, imap.pi.IMAPFolderName, IMAP_STATUS_QUERY)
		}

		go imap.doRequestResponse(command, responseCh, responseErrCh)
		select {
		case <-requestTimer.C:
			// request timed out. Start over.
			imap.Info("Request timed out. Starting over|msgCode=IMAP_POLL_REQ_TIMEDOUT")
			requestTimer.Stop()
			imap.cancelIDLE()

		case err := <-responseErrCh:
			imap.Warning("Got error %s. Sending back LongPollReRegister|msgCode=IMAP_ERR_REREGISTER", err)
			errCh <- LongPollReRegister // erroring out... ask for reregister
			return

		case <-responseCh:
			if imap.hasNewEmail {
				imap.Info("Got mail. Sending LongPollNewMail|msgCode=IMAP_NEW_EMAIL")
				imap.hasNewEmail = false
				errCh <- LongPollNewMail
				return
			}

		case <-stopPollCh: // parent will close this, at which point this will trigger.
			imap.Info("Was told to stop. Stopping")
			return

		case <-stopAllCh: // parent will close this, at which point this will trigger.
			imap.Info("Was told to stop (allStop). Stopping")
			return
		}
	}
}

func (imap *IMAPClient) cancelIDLE() {
	if imap.isIdling {
		imap.Info("Cancelling outstanding IDLE request")
		err := imap.sendIMAPCommand(IMAP_DONE)
		if err != nil {
			imap.Warning("Error sending IMAP command %s while cancelling IDLE request: %s", IMAP_DONE, err)
		}
	}
}
func (imap *IMAPClient) cancel() {
	imap.mutex.Lock()
	imap.cancelled = true
	if imap.tlsConn != nil {
		imap.cancelIDLE()
		imap.tlsConn.Close()
		imap.tlsConn = nil
	}
	imap.mutex.Unlock()
}

func (imap *IMAPClient) Cleanup() {
	imap.Debug("Cleaning up")
	imap.cancel()
	imap.pi.cleanup()
	imap.pi = nil
}
