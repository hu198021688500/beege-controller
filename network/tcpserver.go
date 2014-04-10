package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

type TcpHandler func(data []byte) error

type TCPServer struct {
	address  string
	handlers map[string]TcpHandler
}

func NewTCPServer(address string) (*TCPServer, error) {
	srv := &TCPServer{
		address:  address,
		handlers: make(map[string]TcpHandler),
	}
	return srv, nil
}

func (this *TCPServer) Run() {
	protoAddrParts := strings.SplitN(this.address, "://", 2)
	ln, err := net.Listen(protoAddrParts[0], protoAddrParts[1])
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go this.worker(conn)
	}

	panic("unreachable")
}

func (this *TCPServer) RegisterHandler(name string, handler TcpHandler) error {
	if _, exist := this.handlers[name]; exist {
		return fmt.Errorf("can't overwrite handler for command %s", name)
	} else {
		this.handlers[name] = handler
	}
	return nil
}

func (this *TCPServer) worker(conn net.Conn) {
	var (
		length     int
		blankIndex int
		err        error
		data       []byte
	)
	for {
		if length, err = readPacketLength(conn); err != nil {
			log.Println("read packet head failure")
			break
		}
		if data, err = readPacketData(conn, length); err != nil {
			log.Println("read packet data failure")
			break
		}

		blankIndex = length - 1
		for ; blankIndex > 0; blankIndex-- {
			if data[blankIndex] == 32 {
				break
			}
		}

		if blankIndex > 0 {
			cmd := string(data[blankIndex+1 : length])
			if handler, exist := this.handlers[cmd]; exist {
				err = handler(data[0:blankIndex])
			} else {
				log.Printf("tcp handler[%s] is not exist\n", cmd)
			}
		} else {
			log.Println("command tail is not found int tcp packet")
		}

		if err != nil {
			conn.Write([]byte{0})
		} else {
			conn.Write([]byte{1})
		}
	}

	conn.Close()
}

func readPacketData(conn net.Conn, m int) (data []byte, err error) {
	data = make([]byte, m)
	for l, n := 0, 0; n < m; {
		l, err = conn.Read(data[n:m])
		if nil != err && io.EOF != err {
			return
		}
		n += l
	}
	return
}

func readPacketLength(conn net.Conn) (int, error) {
	head, err := readPacketData(conn, 2)
	if err != nil {
		return 0, err
	} else {
		return int(binary.BigEndian.Uint16(head)), nil
	}
}
