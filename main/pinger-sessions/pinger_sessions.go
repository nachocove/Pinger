package main

import (
	"flag"
	"fmt"
	"github.com/nachocove/Pinger/Pinger"
	"os"
	"path"
)

var usage = func() {
	fmt.Printf("USAGE: %s <flags> <connection string>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
	fmt.Printf("\n  If no '-client', '-context', or '-device' is given, all active sessions are returned.\n")
}

func main() {
	var help bool
	var debug bool
	var verbose bool
	var configFile string
	var clientId string
	var clientContext string
	var deviceId string
	var singleLine bool

	flag.BoolVar(&debug, "d", false, "Debugging")
	flag.BoolVar(&verbose, "v", false, "Verbose")
	flag.BoolVar(&help, "h", false, "Help")
	flag.StringVar(&configFile, "c", "", "The configuration file. Required.")

	flag.StringVar(&clientId, "client", "", "The Client ID to search for.")
	flag.StringVar(&clientContext, "context", "", "The Client Context to search for.")
	flag.StringVar(&deviceId, "device", "", "The Device ID to search for.")
	flag.BoolVar(&singleLine, "s", false, "Write results on a single line for easier grepping. Field delimiter is ';'")

	flag.Parse()
	if help {
		usage()
		os.Exit(0)
	}

	rpcConnectString := os.Getenv("PINGER_RPC")
	
	if rpcConnectString == "" {
		if configFile == "" {
			configFile = os.Getenv("PINGER_CONFIG")
		}
		if configFile != "" {
			config, err := Pinger.ReadConfig(configFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Reading config: %s\n", err)
				os.Exit(1)
			}
			rpcConnectString = config.Rpc.ConnectString()
		}
	}
	if rpcConnectString == "" {
		fmt.Fprintf(os.Stderr, "No Rpc Connect string found. Set environment PINGER_RPC or use -c\n")
		os.Exit(1)
	}
	
	if debug {
		fmt.Fprintf(os.Stdout, "Contacting RPC server at %s\n", rpcConnectString)
		fmt.Fprintf(os.Stdout, "Arguments: ClientId:%s, ClientContext:%s, DeviceId:%s\n", clientId, clientContext, deviceId)
	}
	reply, err := Pinger.FindActiveSessions(rpcConnectString, clientId, clientContext, deviceId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not call FindActiveSessions: %s\n", err)
		os.Exit(1)
	}
	if debug {
		fmt.Fprintf(os.Stdout, "Reply is %+v\n", reply)
	}
	switch {
	case reply.Code == Pinger.PollingReplyError:
		fmt.Fprintf(os.Stderr, "Error fetching sessions: %s\n", reply.Message)
		os.Exit(1)

	case reply.Code == Pinger.PollingReplyOK || reply.Code == Pinger.PollingReplyWarn:
		if reply.Code == Pinger.PollingReplyWarn {
			fmt.Fprintf(os.Stdout, "Warning: %s\n", reply.Message)
		}
		if verbose {
			if singleLine {
				fmt.Fprintf(os.Stdout, "ClientId;ClientContext;DeviceId;Url;Status;Error\n")
			} else {
				fmt.Fprintf(os.Stdout, "Found %d sessions.\n", len(reply.SessionInfos))
			}
		}
		for _, info := range reply.SessionInfos {
			if singleLine {
				fmt.Fprintf(os.Stdout, "%s;%s;%s;%s;%s;%s\n",
					info.ClientId, info.ClientContext, info.DeviceId, info.Url, info.Status, info.Error)
			} else {
				fmt.Fprintf(os.Stdout, "ClientID:%s\nClientContext:%s\nDeviceId:%s\nUrl:%s\nStatus:%s\n",
					info.ClientId, info.ClientContext, info.DeviceId, info.Url, info.Status)
				if info.Status == Pinger.MailClientStatusError {
					fmt.Fprintf(os.Stdout, "Error:%s\n", info.Error)
				}
				fmt.Fprintf(os.Stdout, "\n")
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown response %+v\n", reply)
		os.Exit(1)
	}
	os.Exit(0)
}
