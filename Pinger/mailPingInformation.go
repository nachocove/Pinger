package Pinger

import (
	"encoding/base64"
	"fmt"
	"github.com/nachocove/Pinger/Utils/AWS"
	"github.com/nachocove/Pinger/Utils/Logging"
)

// TODO This should probably move into the MailClient interface/struct

// MailPingInformation the bag of information we get from the client. None of this is saved in the DB.
type MailPingInformation struct {
	ClientId              string
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
	CommandTerminator      []byte // used by imap
	CommandAcknowledgement []byte // used by imap
	ResponseTimeout        int64  // in milliseconds
	WaitBeforeUse          int64  // in milliseconds
	PushToken              string // platform dependent push token
	PushService            string // APNS, AWS, GCM, etc.
	MaxPollTimeout         int64  // max polling lifetime in milliseconds. Default 2 days.
	OSVersion              string
	AppBuildVersion        string
	AppBuildNumber         string
	SessionId              string

	logPrefix string
}

func (pi *MailPingInformation) String() string {
	return fmt.Sprintf("NoChangeReply:%s, RequestData:%s, ExpectedReply:%s",
		base64.StdEncoding.EncodeToString(pi.NoChangeReply),
		base64.StdEncoding.EncodeToString(pi.RequestData),
		base64.StdEncoding.EncodeToString(pi.ExpectedReply))
}

func (pi *MailPingInformation) cleanup() {
	pi.ClientId = ""
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
	pi.CommandTerminator = nil
	pi.CommandAcknowledgement = nil
	pi.PushToken = ""
	pi.PushService = ""
	pi.OSVersion = ""
	pi.AppBuildNumber = ""
	pi.AppBuildVersion = ""
}

// Validate validate the structure/information to make sure required information exists.
func (pi *MailPingInformation) Validate() bool {
	if pi.ClientId == "" || pi.MailServerUrl == "" {
		return false
	}
	switch {
	case pi.Protocol == MailClientActiveSync:
		if len(pi.RequestData) <= 0 || len(pi.HttpHeaders) <= 0 {
			return false
		}
	case pi.Protocol == MailClientIMAP:
		// not yet supported
		return false

	default:
		// unknown protocols are never supported
		return false
	}
	return true
}

func (pi *MailPingInformation) getLogPrefix() string {
	if pi.logPrefix == "" {
		pi.logPrefix = fmt.Sprintf("%s:%s:%s:%s", pi.DeviceId, pi.ClientId, pi.ClientContext, pi.SessionId)
	}
	return pi.logPrefix
}

func (pi *MailPingInformation) newDeviceInfo(db DeviceInfoDbHandler, aws AWS.AWSHandler, logger *Logging.Logger) (*DeviceInfo, error) {
	var err error
	di, err := getDeviceInfo(db, aws, pi.ClientId, pi.ClientContext, pi.DeviceId, pi.SessionId, logger)
	if err != nil {
		return nil, err
	}
	if di == nil {
		di, err = newDeviceInfo(
			pi.ClientId,
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
		_, err := di.updateDeviceInfo(pi.ClientContext, pi.DeviceId, pi.PushService, pi.PushToken, pi.Platform, pi.OSVersion, pi.AppBuildVersion, pi.AppBuildNumber)
		if err != nil {
			return nil, err
		}
	}
	return di, nil
}

