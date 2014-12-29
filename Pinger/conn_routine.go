package Pinger

import (
	"net"
)

// ServerConfiguration - The structure of the json config needed for server values, like port, and bind_address
type connChannels struct {
	Conn net.Conn
	Ch   chan []byte
	ECh  chan error
}

func MakeChan(conn net.Conn) *connChannels {
	ch := make(chan []byte)
	eCh := make(chan error)

	// Start a goroutine to read from our net connection
	go func(ch chan []byte, eCh chan error) {
		data := make([]byte, 512)
		for {
			// try to read the data
			_, err := conn.Read(data)
			if err != nil {
				// send an error if it's encountered
				eCh <- err
				return
			}
			// send data if we read some.
			ch <- data
		}
	}(ch, eCh)

	return &connChannels{conn, ch, eCh}
}
