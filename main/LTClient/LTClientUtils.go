package main

import (
	"crypto/tls"
	"fmt"
	"github.com/nachocove/Pinger/Utils"
	"github.com/nachocove/Pinger/Utils/Logging"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	deferTimeout    = 5       // how long to wait before calling defer in seconds
	postDeferBGTime = 20      // how long to wait after defer timeout to reconnect in seconds
	requestTimeout  = 30 * 60 // how long to wait before timeout
)

const (
	MailClientActiveSync = "ActiveSync"
	MailClientIMAP       = "IMAP"
)

type LTUser struct {
	userName      string
	buffer        []byte
	waitGroup     *sync.WaitGroup
	accountCount  int
	deferCount    int
	testDuration  int
	pingerURL     string
	reopenOnClose bool
	tlsConfig     *tls.Config
	tcpKeepAlive  int
	logger        *Logging.Logger
	stats         *Utils.StatLogger
	stopAllCh     chan int
}

type LTAccount struct {
	user              *LTUser
	accountId         int
	currentDeferCount int
	tlsConn           *tls.Conn
	totalRequestCount int
	accountName       string
	passwd            string
	emailServerName   string
	serverType        string
	transport         *http.Transport
	request           *http.Request
	httpClient        *http.Client
	mockClient        MockClientInterface
	token             string
	logger            *Logging.Logger
}

func (ltu *LTUser) String() string {
	return fmt.Sprintf("LTUser %s", ltu)
}

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func (lta *LTAccount) getSleepTime() int {
	if lta.currentDeferCount == lta.user.deferCount { //out of defers
		return postDeferBGTime
	}
	return deferTimeout
}

func (lta *LTAccount) incrementCounts() {
	if lta.currentDeferCount == lta.user.deferCount { //out of defers
		lta.currentDeferCount = 0
	} else {
		lta.currentDeferCount += 1
	}
	lta.totalRequestCount += 1
}

func (lta *LTAccount) doStop() error {
	logger.Debug("Sending STOP request")
	return lta.mockClient.Stop()
}

func (lta *LTAccount) doRegister() error {
	logger.Debug("Sending REGISTER request")
	return lta.mockClient.Register()
}

func (lta *LTAccount) doDefer() error {
	logger.Debug("Sending DEFER request")
	return lta.mockClient.Defer()
}

func (lta *LTAccount) setupHttp() {
	lta.transport = &http.Transport{
		TLSClientConfig:       lta.user.tlsConfig,
		ResponseHeaderTimeout: time.Duration(requestTimeout) * time.Millisecond,
	}

	lta.httpClient = &http.Client{
		Transport: lta.transport,
	}
}

func (lta *LTAccount) StartAccountSimulation() {
	logger.Info("Starting account simulation")
	defer func() {
		logger.Info("Stopping account simulation.")
		lta.user.waitGroup.Done()
	}()
	lta.setupHttp()
	sleepTime := 0
	logger.Info("Starting test duration timer for %d minutes.", lta.user.testDuration)
	accountTimer := time.NewTimer(time.Duration(lta.user.testDuration) * time.Minute)
	for {
		logger.Debug("a:%s s:%d dc:%d rc:%d", lta.accountName, sleepTime, lta.currentDeferCount, lta.totalRequestCount)
		if sleepTime > 0 {
			s := time.Duration(sleepTime) * time.Second
			logger.Info("Sleeping %s seconds", s)
			time.Sleep(s)
		}
		sleepTime = lta.getSleepTime()
		var err error
		if lta.currentDeferCount == 0 {
			err = lta.doRegister()
		} else {
			err = lta.doDefer()
		}
		if err != nil {
			logger.Info("Post error: %s", err)
			return
		}
		lta.incrementCounts()
		select {
		case <-accountTimer.C:
			// request timed out. Start over.
			logger.Info("Account timed out. Sending stop.")
			accountTimer.Stop()
			err = lta.doStop()
			if err != nil {
				logger.Info("Post error: %s", err)
			}
			return

		case <-lta.user.stopAllCh:
			// parent will close this, at which point this will trigger.
			logger.Info("Was told to stop. Sending stop")
			err = lta.doStop()
			if err != nil {
				logger.Info("Post error: %s", err)
			}
			return
		default:
		}
	}
}

func (ltu *LTUser) StartUserSimulation() error {
	// for each account for the user
	for i := 0; i < ltu.accountCount; i++ {

		accountName := ltu.userName + strconv.Itoa(i)
		logger.Info("Setting up account %s", accountName)
		ltu.waitGroup.Add(1)
		//emailServerName := getRandomDomainName()
		emailServerName := "ltmail.officeburrito.com"
		serverType := getRandomServerType()
		logger.Debug("serverType %s", serverType)
		var mockClient MockClientInterface
		if serverType == MailClientIMAP {
			mockClient = &MockIMAPClient{}
		} else {
			mockClient = &MockEASClient{}
		}
		ltAccount := LTAccount{user: ltu, accountId: i, logger: ltu.logger, accountName: accountName,
			passwd: accountName, emailServerName: emailServerName, serverType: serverType,
			mockClient: mockClient}
		mockClient.init(&ltAccount)
		go ltAccount.StartAccountSimulation()
		Utils.ActiveClientCount++
	}
	return nil
}

func getRandomAround(num int) int {
	return rng.Intn(num) + num/2
}

func getRandomServerType() string {
	if rng.Intn(2) == 0 {
		return MailClientActiveSync
	} else {
		return MailClientIMAP
	}
}

func getRandomUserName() string {
	n := 7
	userName := make([]byte, n)
	for i, v := range rng.Perm(26)[:n] {
		userName[i] = 'a' + byte(v)
	}
	return string(userName)
}

func getRandomDomainName() string {
	n := 10
	domainName := make([]byte, n)
	for i, v := range rng.Perm(26)[:n] {
		domainName[i] = 'a' + byte(v)
	}
	return ("mail" + string(domainName) + "ltmail.officeburrito.com")
}

func NewLTUser(pingerURL string, reopenConnection bool, tcpKeepAlive int,
	tlsConfig *tls.Config, averageAccountCount int, averageDeferCount int,
	testDuration int, stopAllCh chan int, waitGroup *sync.WaitGroup, logger *Logging.Logger) *LTUser {
	userName := getRandomUserName()
	//accountCount := getRandomAround(averageAccountCount)
	accountCount := averageAccountCount
	//deferCount := getRandomAround(averageDeferCount)
	deferCount := averageDeferCount
	// make a random number of defer count about 10

	return &LTUser{
		pingerURL:     pingerURL,
		userName:      userName,
		waitGroup:     waitGroup,
		reopenOnClose: reopenConnection,
		tlsConfig:     tlsConfig,
		tcpKeepAlive:  tcpKeepAlive,
		logger:        logger,
		stopAllCh:     stopAllCh,
		accountCount:  accountCount,
		deferCount:    deferCount,
		testDuration:  testDuration,
		stats:         nil,
	} //Utils.NewStatLogger(stopAllCh, logger, true)
}

// Done The LTUser is exiting. Cleanup and alert anyone waiting.
func (ltUser *LTUser) Done() {
	logger.Info("Finished with User")
}
