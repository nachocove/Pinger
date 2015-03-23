package HostId

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
)

var hostId string

func HostId() string {
	if hostId == "" {
		interfaces, _ := net.Interfaces()
		for _, inter := range interfaces {
			if inter.Name[0:2] == "lo" {
				continue
			}
			if inter.HardwareAddr.String() == "" {
				continue
			}
			hash := sha256.New()
			hash.Write(inter.HardwareAddr)
			md := hash.Sum(nil)
			hostId = hex.EncodeToString(md)
		}
	}
	return hostId
}