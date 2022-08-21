package rtmp

import (
	"bufio"
	"fmt"
	"log"
	"net"

	"github.com/alipourhabibi/restream/grpcclient"
	protos "github.com/alipourhabibi/restream/protos/usersinfo"
	"github.com/alipourhabibi/restream/settings"
)

// Stream is the entrypoint for handling incoming rtmp request for streaming
type Stream struct {
	log *log.Logger
	RPC protos.UsersInfoClient
}

// NewStream returns Steam struct which is for starting streaming service
func NewStream(log *log.Logger) *Stream {
	uInfo := grpcclient.NewUsersInfo(log)
	client := uInfo.GetClient()
	return &Stream{
		log: log,
		RPC: client,
	}
}

// InitStream is  where we start server for streamming
func (s *Stream) InitStream() {

	var ln net.Listener
	var err error
	laddr := fmt.Sprintf(":%d", settings.ServerSettings.Items.Port)
	ln, err = net.Listen("tcp", laddr)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Printf(err.Error())
			continue
		}

		ctx := &StreamContext{}
		ctx.sessions = make(map[string]*Connection)

		c := &Connection{
			log:               s.log,
			Conn:              conn,
			Reader:            bufio.NewReader(conn),
			Writer:            bufio.NewWriter(conn),
			ReadBuffer:        make([]byte, 5096),
			WriteBuffer:       make([]byte, 5096),
			csMap:             make(map[uint32]*rtmpChunk),
			RPC:               s.RPC,
			ReadMaxChunkSize:  128,
			WriteMaxChunkSize: 4096,
			Stage:             handshakeStage,
			Context:           ctx,
		}
		go c.Handle()
	}
}
