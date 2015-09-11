package Pinger

import (
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
)

// TODO This should probably move into the MailClient interface/struct

// MailPingInformation the bag of information we get from the client. None of this is saved in the DB.
type MailPingInformation struct {
	UserId                string
	ClientContext         string
	DeviceId              string
	Platform              string
	MailServerUrl         string
	MailServerCredentials struct {
		Username string
		Password string
	}
	Protocol               string
	HttpHeaders            map[string]string // optional
	RequestData            []byte
	ExpectedReply          []byte
	NoChangeReply          []byte
	ResponseTimeout        uint64 // in milliseconds
	WaitBeforeUse          uint64 // in milliseconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.
	MaxPollTimeout         uint64 // max polling lifetime in milliseconds. Default 2 days.
	OSVersion              string
	AppBuildVersion        string
	AppBuildNumber         string
	SessionId              string
	IMAPAuthenticationBlob string
	IMAPFolderName         string
	IMAPSupportsIdle       bool
	IMAPSupportsExpunge    bool
	IMAPEXISTSCount        uint32
	IMAPUIDNEXT            uint32

	logPrefix string
}

func (pi *MailPingInformation) String() string {
	return fmt.Sprintf("UserId=%s|ClientContext=%s|DeviceId=%s|Platform=%s|MailServerUrl=%s|"+
		"Protocol=%s|ResponseTimeout=%d|WaitBeforeUse=%d|PushToken=%s|PushServer=%s|MaxPollTimeout=%d|"+
		"OSVersion=%s|AppBuildVersion=%s|AppBuildNumber=%s|SessionId=%s|IMAPFolderName=%s|IMAPSupportsIdle=%t|"+
		"IMAPSupportsExpunge=%t|IMAPEXISTSCount=%d|IMAPUIDNEXT=%d",
		pi.UserId, pi.ClientContext, pi.DeviceId, pi.Platform, pi.MailServerUrl, pi.Protocol,
		pi.ResponseTimeout, pi.WaitBeforeUse, pi.PushToken, pi.PushService, pi.MaxPollTimeout, pi.OSVersion,
		pi.AppBuildVersion, pi.AppBuildNumber, pi.SessionId, pi.IMAPFolderName, pi.IMAPSupportsIdle,
		pi.IMAPSupportsExpunge, pi.IMAPEXISTSCount, pi.IMAPUIDNEXT)
}

func (pi *MailPingInformation) cleanup() {
	// TODO investigate if there's a way to memset(0x0) these fields, instead of
	// relying on the garbage collector to clean them up (i.e. assigning "" to them
	// really just moves the pointer, orphaning the previous string, which the garbage
	// collector them frees or reuses.
	pi.UserId = ""
	pi.ClientContext = ""
	pi.DeviceId = ""
	pi.Platform = ""
	pi.MailServerUrl = ""
	pi.MailServerCredentials.Password = ""
	pi.MailServerCredentials.Username = ""
	pi.Protocol = ""
	for k := range pi.HttpHeaders {
		delete(pi.HttpHeaders, k)
	}
	pi.RequestData = nil
	pi.ExpectedReply = nil
	pi.NoChangeReply = nil
	pi.PushToken = ""
	pi.PushService = ""
	pi.OSVersion = ""
	pi.AppBuildNumber = ""
	pi.AppBuildVersion = ""
	pi.IMAPAuthenticationBlob = ""
	pi.IMAPFolderName = ""
	pi.IMAPSupportsIdle = false
	pi.IMAPSupportsExpunge = false
	pi.IMAPEXISTSCount = 0
	pi.IMAPUIDNEXT = 0
}

// Validate validate the structure/information to make sure required information exists.
func (pi *MailPingInformation) Validate() bool {
	// TODO more checking of all fields, since this is all 'user input', including URL for sanity
	// TODO Check the sanity of the Expected replies. Perhaps use some 'reasonable' max?
	if pi.UserId == "" || pi.MailServerUrl == "" {
		return false
	}
	switch {
	case pi.Protocol == MailClientActiveSync:
		if len(pi.RequestData) <= 0 || len(pi.HttpHeaders) <= 0 {
			return false
		}
	case pi.Protocol == MailClientIMAP:
		if len(pi.IMAPAuthenticationBlob) <= 0 || len(pi.IMAPFolderName) <= 0 {
			return false
		}
		return true

	default:
		// unknown protocols are never supported
		return false
	}
	return true
}

func (pi *MailPingInformation) getLogPrefix() string {
	if pi.logPrefix == "" {
		pi.logPrefix = fmt.Sprintf("|device=%s|client=%s|context=%s|session=%s", pi.DeviceId, pi.UserId, pi.ClientContext, pi.SessionId)
	}
	return pi.logPrefix
}

func (pi *MailPingInformation) newDeviceInfo(db DeviceInfoDbHandler, aws AWS.AWSHandler, logger *Logging.Logger) (*DeviceInfo, error) {
	var err error
	di, err := getDeviceInfo(db, aws, pi.UserId, pi.ClientContext, pi.DeviceId, pi.SessionId, logger)
	if err != nil {
		return nil, err
	}
	if di == nil {
		di, err = newDeviceInfo(
			pi.UserId,
			pi.ClientContext,
			pi.DeviceId,
			pi.PushToken,
			pi.PushService,
			pi.Platform,
			pi.OSVersion,
			pi.AppBuildVersion,
			pi.AppBuildNumber,
			pi.SessionId,
			aws,
			db,
			logger)
		if err != nil {
			return nil, err
		}
		if di == nil {
			return nil, fmt.Errorf("Could not create DeviceInfo")
		}
		err = db.insert(di)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := di.updateDeviceInfo(pi.PushService, pi.PushToken, pi.Platform, pi.OSVersion, pi.AppBuildVersion, pi.AppBuildNumber)
		if err != nil {
			return nil, err
		}
	}
	dc, err := di.getContactInfoObj(true)
	if err != nil {
		return nil, err
	}
	if dc == nil {
		panic("Could not create DeviceContact record")
	}
	return di, nil
}
