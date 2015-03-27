package Pinger

import (
	"fmt"
	"net"
	"strings"
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
