package Pinger

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	DefaultPort           = 443
	DefaultBindAddress    = "0.0.0.0"
	DefaultTemplateDir    = "templates"
	DefaultDebugging      = false
	DefaultServerCertFile = ""
	DefaultServerKeyFile  = ""
	DefaultNonTLSPort     = 80
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type ServerConfiguration struct {
	Port             int
	BindAddress      string
	TemplateDir      string
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
		TemplateDir:    DefaultTemplateDir,
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

func (cfg *ServerConfiguration) CreateAuthToken(clientId, clientContext, deviceId string) (string, error) {
	block, err := aes.NewCipher([]byte(cfg.TokenAuthKey))
	if err != nil {
		return "", err
	}
	str := fmt.Sprintf("%d::%s::%s::%s", time.Now().UTC().Unix(), clientId, clientContext, deviceId)
	ciphertext := make([]byte, aes.BlockSize+len(str))
	iv := ciphertext[:aes.BlockSize]
	// TODO Check the RNG algorithm
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	// TODO Check this code for forgeability. Make sure it's encrypted and auth'd (hmac + encryption)
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(str))
	b64 := base64.StdEncoding.EncodeToString(ciphertext)
	return b64, nil
}

func (cfg *ServerConfiguration) ValidateAuthToken(clientId, clientContext, deviceId, token string) (time.Time, error) {
	// TODO Check length on token so base64 decoding doesn't blow up
	errTime := time.Time{}
	block, err := aes.NewCipher([]byte(cfg.TokenAuthKey))
	if err != nil {
		return errTime, err
	}
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return errTime, err
	}
	if len(data) < aes.BlockSize {
		return errTime, fmt.Errorf("ciphertext too short")
	}
	// do a sanity check on the string.
	if len(data) > 512 {
		return errTime, fmt.Errorf("data exceeds acceptable limits")
	}

	iv := []byte(data[:aes.BlockSize])
	text := []byte(data[aes.BlockSize:])
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	if err != nil {
		return errTime, err
	}

	parts := strings.Split(string(text), "::")
	if len(parts) != 4 {
		return errTime, fmt.Errorf("Bad tokenized string %s", string(text))
	}
	var timestamp int64
	n, err := fmt.Sscanf(parts[0], "%d", &timestamp)
	if err != nil {
		return errTime, err
	}
	if n == 0 {
		return errTime, fmt.Errorf("time is empty")
	}
	if parts[1] == "" {
		return errTime, fmt.Errorf("clientID is empty")
	}
	if parts[2] == "" {
		return errTime, fmt.Errorf("context is empty")
	}
	if parts[3] == "" {
		return errTime, fmt.Errorf("deviceId is empty")
	}
	if parts[1] != clientId || parts[2] != clientContext || parts[3] != deviceId {
		return errTime, fmt.Errorf("device info doesn't match token")
	}
	return time.Unix(timestamp, 0), nil
}
