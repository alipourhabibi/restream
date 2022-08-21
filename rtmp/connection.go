package rtmp

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/alipourhabibi/restream/amf"
	protos "github.com/alipourhabibi/restream/protos/usersinfo"
	"github.com/nareix/joy4/format/flv/flvio"
	"github.com/nareix/joy4/utils/bits/pio"
)

// The stage
const (
	_ = iota
	handshakeStage
	commandStage
	commandStageDone
)

// Parse the State of RTMP
const (
	RtmpParseInit = iota
	RtmpParseBasicHeader
	RtmpParseMessageHeader
	RtmpParseExtendedHeader
	RtmpParsePayload
)

type Channel struct {
	ChannelName string
	Send        chan []byte
	Exit        chan bool
}

type UserChannel struct {
	Name string
	URL  string
	Key  string
}

type Services struct {
	Services []Service `json:"services"`
}

type Service struct {
	Name    string    `json:"Name"`
	Servers []Servers `json:"servers"`
}

type Servers struct {
	Name string `json:"Name"`
	URL  string `json:"url"`
}

// For setting and getting Connection based on the streamKey
type StreamContext struct {
	sessions map[string]*Connection
}

func (ctx *StreamContext) set(key string, c *Connection) {
	ctx.sessions[key] = c
}

func (ctx *StreamContext) get(key string) *Connection {
	return ctx.sessions[key]
}

// Connection struct for each conneciton which holds its data
// such as sending and recieving datas
type Connection struct {
	log               *log.Logger
	Conn              net.Conn
	Reader            *bufio.Reader
	Writer            *bufio.Writer
	ReadBuffer        []byte
	WriteBuffer       []byte
	csMap             map[uint32]*rtmpChunk
	ReadMaxChunkSize  int
	WriteMaxChunkSize int
	StreamKey         string
	Stage             int
	Clients           []Channel
	WaitingClient     []Channel
	GotMessage        bool
	MetaData          []byte
	FirstAudio        []byte
	FirstVideo        []byte
	AppName           string
	ConnectionDone    bool
	Streams           int
	RPC               protos.UsersInfoClient
	GotFirstAudio     bool
	GotFirstVideo     bool
	Context           *StreamContext
}

// Handle each connection recieved
func (c *Connection) Handle() {
	if err := c.handshake(); err != nil {
		c.log.Println(err.Error())
		return
	}
	if err := c.prepare(); err != nil {
		c.log.Println(err.Error())
		return
	}
	// Connectoin Completed

	for c.Stage < commandStageDone {
		c.readMessage()
	}
	// CommandStage Completed

	for {
		if err := c.readChunk(); err != nil {
			c.closeConnection()
			return
		}
	}
}

func (c *Connection) prepare() error {
	for {
		err := c.readMessage()
		if err != nil {
			return err
		}
		if c.ConnectionDone {
			return err
		}
	}
}

func (c *Connection) readMessage() (err error) {
	c.GotMessage = false
	for {
		if err = c.readChunk(); err != nil {
			return
		}
		if c.GotMessage {
			return
		}
	}
}

func (c *Connection) readChunk() error {
	var bytesRead int

	// Read the first byte of data to determine csid and fmt
	// For more info refer to the https://en.wikipedia.org/wiki/Real-Time_Messaging_Protocol#Packet_structure
	if _, err := io.ReadFull(c.Reader, c.ReadBuffer[:1]); err != nil {
		return err
	}
	bytesRead++

	// 0x3f => '0b00111111' will be &(AND) with first byte
	// csid is the the least significant bits of first byte
	csid := uint32(c.ReadBuffer[0]) & 0x3f

	// fmt is the 2 most significant bits of first byte
	// to get this it should be shifted 6 bits
	_fmt := uint8(c.ReadBuffer[0] >> 6)

	// refer to the link given above
	// csid = 0 => csid is csid
	if csid == 0 {
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+1]); err != nil {
			return err
		}
		// if csid is 0 then the BH(Basic Header) is 2 bytes
		// and is 64 + csid
		csid = uint32(c.ReadBuffer[bytesRead]) + 64
		bytesRead++
	} else if csid == 1 {
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+2]); err != nil {
			return err
		}
		// 64 + first byte in uint32 + second byte * 256 which is 2**8
		csid = ((uint32(c.ReadBuffer[bytesRead+1]) * 256) + uint32(c.ReadBuffer[bytesRead]+64))

		// optionaly you can use LitteleEndian for this matter
		// csid = uint32(binary.LittleEndian.Uint16(c.ReadBuffer[bytesRead:bytesRead+2])) + 64

		bytesRead += 2
	}
	// if csid is 2 then we don't change it

	// Pick the previos chunk of smae chunk streamID
	// to check if it is a new chunk or continiue of
	// a previos chunk
	chunk, ok := c.csMap[csid]
	// if it is new chunk
	if !ok {
		chunk = c.createRtmpChunk(_fmt, csid)
	}

	// if fmt = 0 => 12 bytes header
	// if fmt = 1 => 8 bytes header
	// if fmt = 2 => 4 bytes header
	// if fmt = 3 => 1 bytes header

	// if _fmt <= 2 then we will surely read 3 bytes which is timestamp
	if _fmt <= 2 {
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+3]); err != nil {
			return err
		}
		// Uint24 BigEndian is the way the timestamp will be sent
		chunk.header.timestamp = pio.U24BE(c.ReadBuffer[bytesRead : bytesRead+3])
		bytesRead += 3
	}
	// if _fmt <= 1 then we have length with 24 bytes in BigEndian format and messageType
	if _fmt <= 1 {
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+4]); err != nil {
			return err
		}
		// Read wikipedia for more info and see diagram for header's items
		chunk.header.length = pio.U24BE(c.ReadBuffer[bytesRead : bytesRead+3])
		chunk.header.messageType = uint8(c.ReadBuffer[bytesRead+3])
		bytesRead += 4
	}
	// if _fmt == 0 then we have messageStreamID in 32 bytes and BigEndian format
	if _fmt == 0 {
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+4]); err != nil {
			return err
		}
		chunk.header.messageStreamID = binary.LittleEndian.Uint32(c.ReadBuffer[bytesRead : bytesRead+4])
		bytesRead += 4
	}

	// if the time stamp is at maximum it means we have extended timestamp
	if chunk.header.timestamp == 0xFFFFFF {
		chunk.header.hasExtendedTimestamp = true
		// Read the extended timestapmp
		if _, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+4]); err != nil {
			return err
		}
		// extended timestamp is in BigEndian format with 32 bytes
		chunk.header.timestamp = binary.BigEndian.Uint32(c.ReadBuffer[bytesRead : bytesRead+4])
		bytesRead += 4
	} else {
		chunk.header.hasExtendedTimestamp = false
	}
	chunk.delta = chunk.header.timestamp
	chunk.clock += chunk.header.timestamp

	// if we don't have payload
	if chunk.bytes == 0 {
		chunk.payload = make([]byte, chunk.header.length)
		chunk.capacity = chunk.header.length
	}

	// if size exeeds the maximum change back size to maximum
	size := int(chunk.header.length) - chunk.bytes
	if size > c.ReadMaxChunkSize {
		size = c.ReadMaxChunkSize
	}

	// read payload
	n, err := io.ReadFull(c.Reader, c.ReadBuffer[bytesRead:bytesRead+size])
	if err != nil {
		return err
	}

	// set payload
	// if it is not the first chunk of the payload it will be appended to the previous one
	chunk.payload = append(chunk.payload[:chunk.bytes], c.ReadBuffer[bytesRead:bytesRead+size]...)
	chunk.bytes += n
	bytesRead += n

	// if we finish handling the commands and we want to send it
	// to the correspondings servers like twitch and...
	if c.Stage == commandStageDone {
		temp := make([]byte, bytesRead)
		copy(temp, c.ReadBuffer[:bytesRead])
		for _, client := range c.Clients {
			client.Send <- temp
		}
	}

	// if we got the whole chunk and we are ready to handle them
	if chunk.bytes == int(chunk.header.length) {
		c.GotMessage = true
		chunk.bytes = 0
		// handle the chunk
		c.handleChunk(chunk)
	}
	// set the chunk to its corresponding csid in csMap
	c.csMap[chunk.header.csid] = chunk
	bytesRead = 0

	return err
}

// handle the chunk based on the given messageType
func (c *Connection) handleChunk(chunk *rtmpChunk) {
	switch chunk.header.messageType {

	// setting the maximum chunk size
	case 1:
		c.ReadMaxChunkSize = int(binary.BigEndian.Uint32(chunk.payload))

	case 5:
	//Window ACK Size

	// handling AMF0 Commands
	case 20:
		c.handleAmf0Commad(chunk)

	case 18:
		c.handleDataMessage(chunk)

	case 8:
		c.handleAudioData(chunk)

	case 9:
		c.handleVidoeData(chunk)

	default:
		c.log.Println("[ERROR] error in handling chunk")
		c.log.Println(chunk.header.messageType)
	}
}

// handle AMF0 Command
// For more info refer to wikipedia page in README.md
func (c *Connection) handleAmf0Commad(chunk *rtmpChunk) {
	command := amf.Decode(chunk.payload)

	switch command["cmd"] {
	case "connect":
		c.onConnect(command)
	case "releaseStream":
		c.onRelease(command)
	case "FCPublish":
		c.onFCPublish(command)
	case "createStream":
		c.onCreateStream(command)
	case "publish":
		c.onPublish(command, chunk.header.messageStreamID)
	case "play":
		c.onPlay(command, chunk)
	case "pause":
	case "FCUnpublish":
	case "deleteStream":
	case "closeStream":
	case "receiveAudio":
	case "receiveVideo":
	default:
		c.log.Println("[ERROR] UNKNOWN AMF0 COMMAND")
	}
}

func (c *Connection) onConnect(command map[string]interface{}) {
	c.AppName = command["cmdObj"].(map[string]interface{})["app"].(string)

	// TODO fix these methods
	c.setMaxWriteChunkSize(128)
	c.sendWindowACK(5000000)
	c.setPeerBandwidth(5000000, 2)
	c.Writer.Flush()

	// setup payload that should be returned to client
	cmd := "_result"
	transID := command["transId"]
	cmdObj := flvio.AMFMap{
		"fmsVer":       "FMS/3,0,1,123",
		"capabilities": 31,
	}
	info := flvio.AMFMap{
		"level":          "status",
		"code":           "NetConnection.Connect.Success",
		"description":    "Connection succeeded",
		"objectEncoding": 0,
	}
	amfPayload, length := amf.Encode(cmd, transID, cmdObj, info)

	chunk := &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            3,
			messageType:     20,
			messageStreamID: 0,
			timestamp:       0,
			length:          uint32(length),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  amfPayload,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()
	c.ConnectionDone = true
}

// Nothing need to be done here write now
func (c *Connection) onRelease(command map[string]interface{}) {
}

// Nothing need to be done here write now
func (c *Connection) onFCPublish(command map[string]interface{}) {
}

func (c *Connection) onCreateStream(command map[string]interface{}) {
	c.Streams++

	cmd := "_result"
	transID := command["transId"]
	cmdObj := interface{}(nil)
	info := c.Streams

	amfPayload, length := amf.Encode(cmd, transID, cmdObj, info)

	chunk := &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            3,
			messageType:     20,
			messageStreamID: 0,
			timestamp:       0,
			length:          uint32(length),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  amfPayload,
	}

	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()
}

func (c *Connection) onPublish(command map[string]interface{}, messageStreamID uint32) {
	cmd := "onStatus"
	transID := 0
	cmdObj := interface{}(nil)
	info := flvio.AMFMap{
		"level":       "status",
		"code":        "NetStream.Publish.Start",
		"description": "Published",
	}

	amfPayload, length := amf.Encode(cmd, transID, cmdObj, info)

	chunk := &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            3,
			messageType:     20,
			messageStreamID: messageStreamID,
			timestamp:       0,
			length:          uint32(length),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  amfPayload,
	}

	key := command["streamName"].(string)
	c.Context.set(key, c)
	c.StreamKey = key
	response, err := c.RPC.Get(context.Background(), &protos.UsersInfoRequest{
		Key: key,
	})
	if err != nil {
		c.log.Println(err.Error())
		c.Conn.Close()
		return
	}
	// if user is not authorized
	if !response.Auth {
		c.Conn.Close()
		return
	}
	jsonDatas, err := os.ReadFile("services/servers.json")
	if err != nil {
		c.log.Println(err.Error())
		return
	}
	jsonMap := Services{}
	json.Unmarshal(jsonDatas, &jsonMap)
	fmt.Printf("%v\n", jsonMap)

	userChannel := []UserChannel{}

	if response.Twitch.GetKey() != "" {
		ch := UserChannel{
			Name: jsonMap.Services[0].Servers[0].Name,
			URL:  jsonMap.Services[0].Servers[0].URL,
			Key:  response.Twitch.GetKey(),
		}
		userChannel = append(userChannel, ch)
	}
	if response.Youtube.GetKey() != "" {
		ch := UserChannel{
			Name: jsonMap.Services[1].Servers[0].Name,
			URL:  jsonMap.Services[1].Servers[0].URL,
			Key:  response.Twitch.GetKey(),
		}
		userChannel = append(userChannel, ch)
	}
	if response.Aparat.GetKey() != "" {
		ch := UserChannel{
			Name: jsonMap.Services[2].Servers[0].Name,
			URL:  jsonMap.Services[2].Servers[0].URL,
			Key:  response.Twitch.GetKey(),
		}
		userChannel = append(userChannel, ch)
	}

	for _, channel := range userChannel {
		ch := Channel{
			ChannelName: channel.Name,
			Send:        make(chan []byte, 100),
			Exit:        make(chan bool, 5),
		}
		c.Clients = append(c.Clients, ch)
		c.prepareClient(channel.URL+"/"+channel.Key, ch)
	}

	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()

	c.Stage++
}

func (c *Connection) closeConnection() {
	for _, client := range c.Clients {
		client.Exit <- true
	}
}

func (c *Connection) handleDataMessage(chunk *rtmpChunk) {
	command := amf.Decode(chunk.payload)

	switch command["cmd"] {
	case "@setDataFrame":
		c.MetaData = append(c.MetaData, chunk.payload...)
	}
}

func (c *Connection) handleAudioData(chunk *rtmpChunk) {
	chunk.header.timestamp = chunk.clock
	if !c.GotFirstAudio {
		c.FirstAudio = append(c.FirstAudio, chunk.payload...)
	}
	c.GotFirstAudio = true
}

func (c *Connection) handleVidoeData(chunk *rtmpChunk) {
	chunk.header.timestamp = chunk.clock
	if !c.GotFirstVideo {
		c.FirstVideo = append(c.FirstVideo, chunk.payload...)
	}
	c.GotFirstVideo = true
	if len(c.WaitingClient) > 0 {
		frameType := chunk.payload[0] >> 4
		if frameType == 1 {
			for i, client := range c.WaitingClient {
				for _, ch := range c.create(chunk) {
					client.Send <- ch
				}
				c.Clients = append(c.Clients, client)
				c.WaitingClient = append(c.WaitingClient[:i], c.WaitingClient[i+1:]...)
			}
		}
	}
}

func (c *Connection) onPlay(command map[string]interface{}, playChunk *rtmpChunk) {
	co := c.Context.get(command["streamName"].(string))
	if co == nil {
		return
	}
	b := make([]byte, 6)
	binary.BigEndian.PutUint16(b[:2], 0)
	binary.BigEndian.PutUint32(b[2:], playChunk.header.messageStreamID)
	chunk := &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            2,
			messageType:     4,
			messageStreamID: 0,
			timestamp:       0,
			length:          uint32(6),
		}, clock: 0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  b,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}

	info := flvio.AMFMap{
		"level":       "status",
		"code":        "NetStream.Play.Start",
		"description": "Start live",
	}
	amfPayload, _ := amf.Encode("onStatus", 4, nil, info)

	chunk = &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            3,
			messageType:     20,
			messageStreamID: playChunk.header.messageStreamID,
			timestamp:       0,
			length:          uint32(len(amfPayload)),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  amfPayload,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}

	amfPayload, _ = amf.Encode("|RtmpSampleAccess", false, false)

	chunk = &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            6,
			messageType:     20,
			messageStreamID: 0,
			timestamp:       0,
			length:          uint32(len(amfPayload)),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  amfPayload,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}

	c.Writer.Flush()
	c.Stage = commandStageDone

	chunk = &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            6,
			messageType:     18,
			messageStreamID: playChunk.header.messageStreamID,
			timestamp:       0,
			length:          uint32(len(co.MetaData)),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  co.MetaData,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()
	// Meta Data Sent

	chunk = &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            4,
			messageType:     8,
			messageStreamID: playChunk.header.messageStreamID,
			timestamp:       0,
			length:          uint32(len(co.FirstAudio)),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  co.FirstAudio,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()
	// Audio Data Sent

	chunk = &rtmpChunk{
		header: &header{
			fmt:             0,
			csid:            4,
			messageType:     9,
			messageStreamID: playChunk.header.messageStreamID,
			timestamp:       0,
			length:          uint32(len(co.FirstVideo)),
		},
		clock:    0,
		delta:    0,
		capacity: 0,
		bytes:    0,
		payload:  co.FirstVideo,
	}
	for _, ch := range c.create(chunk) {
		c.Writer.Write(ch)
	}
	c.Writer.Flush()
	// Video Data Sent

	ch := Channel{
		ChannelName: "-1",
		Send:        make(chan []byte, 100),
		Exit:        make(chan bool, 5),
	}
	co.WaitingClient = append(co.WaitingClient, ch)

	func(client *Connection) {
		clientWriter := bufio.NewWriter(c.Conn)
		for {
			select {
			case chunk := <-ch.Send:
				clientWriter.Write(chunk)
			case <-ch.Exit:
				client.Conn.Close()
				return
			}
			clientWriter.Flush()
		}
	}(c)
}
