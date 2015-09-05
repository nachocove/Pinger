package Pinger

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
)

const (
	DefaultPort           = 443
	DefaultBindAddress    = "0.0.0.0"
	DefaultDebugging      = false
	DefaultServerCertFile = ""
	DefaultServerKeyFile  = ""
	DefaultNonTLSPort     = 0
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type ServerConfiguration struct {
	Port             int
	BindAddress      string
	TemplateDir      string // deprecated. If removed, all existing configs need to be fixed.
	ServerCertFile   string
	ServerKeyFile    string
	NonTlsPort       int      `gcfg:"non-tls-port"`
	SessionSecret    string   `gcfg:"session-secret"`
	AliveCheckIPList []string `gcfg:"alive-check-ip"`
	AliveCheckToken  []string `gcfg:"alive-check-token"`
	DumpRequests     bool
	Debug            bool
	TokenAuthKey     string

	aliveCheckCidrList []*net.IPNet `gcfg:"-"`
}

func NewServerConfiguration() *ServerConfiguration {
	return &ServerConfiguration{
		Port:           DefaultPort,
		BindAddress:    DefaultBindAddress,
		ServerCertFile: DefaultServerCertFile,
		ServerKeyFile:  DefaultServerKeyFile,
		NonTlsPort:     DefaultNonTLSPort,
		SessionSecret:  "",
		TokenAuthKey:   "",
	}
}
func (cfg *ServerConfiguration) validate() error {
	badIP := make([]string, 0, 5)
	for _, cidr := range cfg.AliveCheckIPList {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			badIP = append(badIP, err.Error())
		} else {
			cfg.aliveCheckCidrList = append(cfg.aliveCheckCidrList, ipnet)
		}
	}
	if len(badIP) > 0 {
		return fmt.Errorf("alive-check-ip: %s", strings.Join(badIP, ", "))
	}
	if cfg.TokenAuthKey == "" {
		return fmt.Errorf("TokenAuthKey can not be empty")
	}
	_, err := aes.NewCipher([]byte(cfg.TokenAuthKey))
	if err != nil {
		return err
	}
	return nil
}

func (cfg *ServerConfiguration) CheckIPListString() string {
	return strings.Join(cfg.AliveCheckIPList, ", ")
}

func (cfg *ServerConfiguration) CheckIP(ip net.IP) bool {
	foundMatch := false
	if len(cfg.aliveCheckCidrList) == 0 {
		foundMatch = true
	} else {
		for _, ipnet := range cfg.aliveCheckCidrList {
			if ipnet.Contains(ip) {
				foundMatch = true
				break
			}
		}
	}
	return foundMatch
}

func (cfg *ServerConfiguration) CheckToken(token string) bool {
	foundMatch := false
	for _, tok := range cfg.AliveCheckToken {
		if strings.EqualFold(tok, token) {
			foundMatch = true
			break
		}
	}
	return foundMatch
}

func (cfg *ServerConfiguration) CreateAuthToken(userId, clientContext, deviceId string) (string, []byte, error) {
	str := fmt.Sprintf("%s:%s:%s", userId, clientContext, deviceId)
	key := make256Key()
	authTokenMAC := makeTokenMAC([]byte(str), key)
	b64Token := base64.StdEncoding.EncodeToString(authTokenMAC)
	return b64Token, key, nil
}

func (cfg *ServerConfiguration) ValidateAuthToken(userId, clientContext, deviceId, tokenb64 string, key []byte) bool {
	// TODO Check length on token so base64 decoding doesn't blow up
	str := fmt.Sprintf("%s:%s:%s", userId, clientContext, deviceId)
	token, err := base64.StdEncoding.DecodeString(tokenb64)
	if err != nil {
		return false
	}
	return checkMAC([]byte(str), token, key)
}

func checkMAC(message, messageMAC, key []byte) bool {
	expectedMAC := makeTokenMAC(message, key)
	return hmac.Equal(messageMAC, expectedMAC)
}

func make256Key() []byte {
	key := make([]byte, 256)
	_, err := rand.Read(key)
	if err != nil {
		return nil
	}
	return key
}

func makeTokenMAC(message, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	token := mac.Sum(nil)
	return token
}
