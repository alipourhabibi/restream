package rtmp

import (
	"encoding/binary"
	"math"

	"github.com/nareix/joy4/utils/bits/pio"
)

type rtmpChunk struct {
	header   *header
	clock    uint32
	delta    uint32
	payload  []byte
	bytes    int
	capacity uint32
}

type header struct {
	fmt                  uint8
	csid                 uint32
	timestamp            uint32
	length               uint32
	messageType          uint8
	messageStreamID      uint32
	hasExtendedTimestamp bool
}

var chunkHeaderSize = map[uint8]int{
	// to read more refer to wikipedia link
	// provided in README.md
	0: 11,
	1: 7,
	2: 3,
	3: 0,
}

func (c *Connection) createRtmpChunk(fmt uint8, csid uint32) *rtmpChunk {
	header := &header{
		fmt:                  fmt,
		csid:                 csid,
		timestamp:            0,
		length:               0,
		messageType:          0,
		messageStreamID:      0,
		hasExtendedTimestamp: false,
	}

	chunk := &rtmpChunk{
		header:   header,
		clock:    0,
		delta:    0,
		bytes:    0,
		capacity: 0,
	}
	return chunk
}

func (c Connection) create(chunk *rtmpChunk) [][]byte {
	basicHeader := chunk.createBasicHeader()
	messageHeader := chunk.createMessageHeader()
	extendedTimestamp := chunk.createExtendedTimestamp()
	payloads := chunk.createPaylaodArray(c)

	chunks := make([][]byte, len(payloads))
	var bytes []byte
	bytes = append(bytes, basicHeader...)
	bytes = append(bytes, messageHeader...)
	bytes = append(bytes, extendedTimestamp...)
	bytes = append(bytes, payloads[0]...)

	chunks[0] = bytes

	for i := 1; i < len(payloads); i++ {
		tempChunk := rtmpChunk{
			header: &header{
				fmt:  3,
				csid: chunk.header.csid,
			},
			payload: payloads[i],
		}
		basicHeader = tempChunk.createBasicHeader()

		chunks[i] = append(basicHeader, payloads[i]...)
	}
	return chunks
}

func (chunk rtmpChunk) createBasicHeader() []byte {
	var res []byte
	// if the csid is 1 then BH is 3 bytes
	if chunk.header.csid >= 64+255 {
		res = make([]byte, 3)
		// the csid is 1 and fmt is 2bit so
		// it we should shift 6 bit and or it
		// with 1(csid)
		res[0] = (chunk.header.fmt << 6) | 1
		// 0xFF => 0b11111111
		// last 2 bytes encoded as LittleEndian
		// we +64 the when we want to calculate csid
		// so we should - it now
		res[1] = uint8((chunk.header.csid - 64) & 0xFF)
		res[2] = uint8(((chunk.header.csid - 64) >> 8) & 0xFF)
		// if csid is not 1 and is 0 then BH is 2 bytes
	} else if chunk.header.csid >= 64 {
		res = make([]byte, 2)
		// csid => 0
		res[0] = (chunk.header.fmt << 6) | 0
		// we +64 the when we want to calculate csid
		// so we should - it now
		res[1] = uint8((chunk.header.csid - 64) & 0xFF)
		// if csid is 2 then BH is the csid itself
	} else {
		res = make([]byte, 1)
		// it should shift 6 bits
		res[0] = (chunk.header.fmt << 6) | uint8(chunk.header.csid)
	}

	return res
}
func (chunk rtmpChunk) createMessageHeader() []byte {
	// setup the messageHeader size based on the fmt
	res := make([]byte, chunkHeaderSize[chunk.header.fmt])

	// we surely have timestamp
	// which is 3 bytes
	if chunk.header.fmt <= 2 {
		if chunk.header.timestamp >= 0xffffff {
			pio.PutU24BE(res, 0xffffff)
			chunk.header.hasExtendedTimestamp = true
		} else {
			pio.PutU24BE(res, chunk.header.timestamp)
		}
	}

	// we surely have length
	// which is 3 bytes and starts from byte 3
	// and we have messageType which is 1 byte
	if chunk.header.fmt <= 1 {
		pio.PutU24BE(res[3:], chunk.header.length)
		res[6] = chunk.header.messageType
	}

	// we have messageStreamId which starts at byte 7
	if chunk.header.fmt == 0 {
		binary.LittleEndian.PutUint32(res[7:], chunk.header.messageStreamID)
	}

	return res
}
func (chunk rtmpChunk) createExtendedTimestamp() []byte {
	// if we have extended timestamp we make 4bytes in BigEndian
	// and set it
	if chunk.header.hasExtendedTimestamp {
		res := make([]byte, 4)
		binary.BigEndian.PutUint32(res, chunk.header.timestamp)
		return res
	}
	// else we return 0 size []byte
	return make([]byte, 0)
}

func (chunk rtmpChunk) createPaylaodArray(c Connection) [][]byte {
	// check the number of chunks
	totalChunks := int(math.Ceil(float64(float64(chunk.header.length) / float64(c.WriteMaxChunkSize))))

	if totalChunks == 0 {
		totalChunks = 1
	}
	payloads := make([][]byte, totalChunks)

	offset := 0
	// each chunk size shouldn't be greater than WriteMaxChunkSize
	for i := 0; i < totalChunks; i++ {
		size := int(chunk.header.length) - offset
		if size > c.WriteMaxChunkSize {
			size = c.WriteMaxChunkSize
		}
		payloads[i] = chunk.payload[offset : offset+size]
		offset += size
	}

	return payloads
}
