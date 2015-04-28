package Pinger

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
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
	aesKey             []byte       `gcfg:"-"`
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
	if cfg.TokenAuthKey != "" {
		fmt.Fprintf(os.Stderr, "TokenAuthKey is deprecated and will be ignored. Please remove it from the config.\n")
	}
	return cfg.initAes()
}

func (cfg *ServerConfiguration) initAes() error {
	if cfg.aesKey == nil {
		aesKey := make([]byte, 32)
		_, err := rand.Read(aesKey)
		if err != nil {
			panic(err)
		}
		cfg.aesKey = aesKey

		// test the key
		_, err = aes.NewCipher(cfg.aesKey)
		if err != nil {
			return err
		}
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
	err := cfg.initAes()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cfg.aesKey)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	blocksize := aes.BlockSize

	str := fmt.Sprintf("%d::%s::%s::%s::", time.Now().UTC().Unix(), clientId, clientContext, deviceId)
	outBlocks := len(str) / blocksize
	if len(str)%blocksize != 0 {
		outBlocks++
	}
	ciphertext := make([]byte, outBlocks*blocksize)
	copy(ciphertext, []byte(str))

	mac := hmac.New(sha256.New, cfg.aesKey)
	mac.Write(ciphertext)
	expectedMAC := mac.Sum(nil)
	b.Write(expectedMAC)

	iv := make([]byte, blocksize)
	// TODO Check the RNG algorithm
	_, err = rand.Read(iv)
	if err != nil {
		return "", err
	}
	b.Write(iv)


	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	b.Write(ciphertext)

	b64 := base64.StdEncoding.EncodeToString(b.Bytes())
	return b64, nil
}

func (cfg *ServerConfiguration) ValidateAuthToken(clientId, clientContext, deviceId, token string) (time.Time, error) {
	errTime := time.Time{}
	if len(token) > 500 {
		return errTime, fmt.Errorf("token length seems excessive")
	}

	err := cfg.initAes()
	if err != nil {
		return errTime, err
	}
	// TODO Check length on token so base64 decoding doesn't blow up
	block, err := aes.NewCipher(cfg.aesKey)
	if err != nil {
		return errTime, err
	}
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return errTime, err
	}

	// do sanity checks on the string.
	if len(data) < 2*aes.BlockSize+sha256.BlockSize {
		return errTime, fmt.Errorf("ciphertext too short")
	}
	if len(data) > 10*aes.BlockSize+sha256.BlockSize {
		return errTime, fmt.Errorf("data exceeds acceptable limits")
	}

	i := 0
	sentHmac := data[0:len(cfg.aesKey)]
	i += len(cfg.aesKey)

	iv := make([]byte, aes.BlockSize)
	copy(iv, data[i:i+aes.BlockSize])
	i += aes.BlockSize

	text := data[i:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(text, text)

	mac := hmac.New(sha256.New, cfg.aesKey)
	mac.Write(text)
	expectedMAC := mac.Sum(nil)

	if !hmac.Equal(sentHmac, expectedMAC) {
		return errTime, fmt.Errorf("Message does not pass the hmac check")
	}

	parts := strings.Split(string(text), "::")
	if len(parts) != 5 { // Why 5? 4 parts plus any encryption-added padding.
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
