package rtmp

import (
	"bufio"
	"encoding/hex"
	"fmt"

	"github.com/praveen001/joy4/format/rtmp"
)

func (c *Connection) setMaxWriteChunkSize(size uint16) {
	buf, _ := hex.DecodeString("02000000000004010000000000001000")
	if _, err := c.Writer.Write(buf); err != nil {
		c.log.Println(err.Error())
	}
}

func (c *Connection) sendWindowACK(size uint32) {
	buf, _ := hex.DecodeString("02000000000004030000000010001000")
	if _, err := c.Writer.Write(buf); err != nil {
		c.log.Println(err.Error())
	}
}

func (c *Connection) setPeerBandwidth(size uint32, limit uint8) {
	buf, _ := hex.DecodeString("0200000000000506000000000100100002")
	if _, err := c.Writer.Write(buf); err != nil {
		c.log.Println(err.Error())
	}
}

func (c *Connection) prepareClient(url string, ch Channel) {
	fmt.Println(url)
	client, err := rtmp.Dial(url)
	if err != nil {
		// Probably invalid Key
		return
	}
	client.Prepare()

	clientWriter := bufio.NewWriter(client.NetConn())

	go func() {
		for {
			select {
			case chunk := <-ch.Send:
				clientWriter.Write(chunk)
			case <-ch.Exit:
				client.WriteTrailer()
				client.Close()
				return
			}
			clientWriter.Flush()
		}
	}()
}
