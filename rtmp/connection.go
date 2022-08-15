package rtmp

import (
	"bufio"
	"log"
	"net"
)

// Connection struct for each conneciton which holds its data
// such as sending and recieving datas
type Connection struct {
	log         *log.Logger
	Conn        net.Conn
	Reader      *bufio.Reader
	Writer      *bufio.Writer
	ReadBuffer  []byte
	WriteBuffer []byte
}

// Handle each connection recieved
func (c *Connection) Handle() {
	if err := c.handshake(); err != nil {
		c.log.Println(err.Error())
		return
	}
}
