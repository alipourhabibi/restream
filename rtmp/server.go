package rtmp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"github.com/alipourhabibi/restream/settings"
)

// Stream is the entrypoint for handling incoming rtmp request for streaming
type Stream struct {
	log *log.Logger
}

// NewStream returns Steam struct which is for starting streaming service
func NewStream(log *log.Logger) *Stream {
	return &Stream{
		log: log,
	}
}

// InitStream is  where we start server for streamming
func (s *Stream) InitStream() {

	var ln net.Listener
	var err error
	runMode := settings.ServerSettings.Items.RunMode
	laddr := fmt.Sprintf(":%d", settings.ServerSettings.Items.Port)

	if runMode == "Release" {
		cert, err := tls.LoadX509KeyPair("certfiles/general/cert.pem", "certfiles/general/key.pem")
		if err != nil {
			panic(err)
		}
		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", laddr, config)
		if err != nil {
			panic(err)
		}
		defer ln.Close()
	} else if runMode == "Debug" {
		ln, err = net.Listen("tcp", laddr)
		if err != nil {
			panic(err)
		}
		defer ln.Close()
	} else {
		fmt.Println("[ERROR] Invalid RunMode")
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Printf(err.Error())
			continue
		}

		c := &Connection{
			log:         s.log,
			Conn:        conn,
			Reader:      bufio.NewReader(conn),
			Writer:      bufio.NewWriter(conn),
			ReadBuffer:  make([]byte, 5096),
			WriteBuffer: make([]byte, 5096),
		}
		go c.Handle()
	}
}
