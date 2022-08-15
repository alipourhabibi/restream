package rtmp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

func (c *Connection) handshake() error {
	// 1 for C0 and S0
	// each 1536 for C1, C2, S1, S2
	var random [(1 + 1536 + 1536) * 2]byte

	// half of the random is for C0C1C2
	var C0C1C2 = random[:(1 + 1536 + 1536)]
	var C0 = C0C1C2[:1]
	var C1 = C0C1C2[1 : 1536+1]
	var C0C1 = C0C1C2[:1536+1]
	var C2 = C0C1C2[1+1536:]

	var S0S1S2 = random[(1 + 1536 + 1536):]
	var S0 = S0S1S2[:1]
	var S1 = S0S1S2[1 : 1536+1]
	var S2 = S0S1S2[1+1536:]

	// Read from c.Reader which is our conneciton to the C0C1
	_, err := io.ReadFull(c.Reader, C0C1)
	if err != nil {
		return err
	}

	// check if rtmp version is correct
	// client will send its rtmp version in the first bit which is C0[0]
	// in here
	if C0[0] != 3 {
		err := fmt.Errorf("Unsupported RTMP version %d", C0[0])
		return err
	}

	// time will be sent through 4 bytes which
	// will be converted to unit32 with bigendian
	// the 4 bytes are the first 4 bytes of C1 which are 1 byte after C0
	// clienttime := binary.BigEndian.Uint32(C1[:4])

	//clientVersion will be sent in 4 bytes
	// which will be converted to unit32 with bigendian
	// the 4 bytes are the second 4 bytes of c1 which are 5 bytes after C0
	clientVersion := binary.BigEndian.Uint32(C1[4:8])

	if clientVersion != 0 {
		// Complex Handshake
		// which we will not implement
		err := fmt.Errorf("Complex Handshake not implemented")
		return err
	}

	// copy verson to the S0 which is what we will send to the client
	copy(S0, C0)

	// create S1 from random to send to the client to complete handshake
	rand.Read(S1)

	// return client's random data to complete handshake
	copy(S2, C1)

	// Send data to the client
	_, err = c.Writer.Write(S0S1S2)
	if err != nil {
		err := fmt.Errorf("Error sending S0S1S0 %s", err.Error())
		return err
	}
	if err = c.Writer.Flush(); err != nil {
		err := fmt.Errorf("Error Flushing S0S1S0 %s", err.Error())
		return err
	}

	// Reading the last part of the handshake which is C2
	if _, err = io.ReadFull(c.Reader, C2); err != nil {
		err := fmt.Errorf("Error reading C2 %s", err.Error())
		return err
	}

	if !reflect.DeepEqual(C2, S1) {
		err := fmt.Errorf("Invalid C2 from client")
		return err
	}

	fmt.Println("handshake DONE")
	return nil
}
