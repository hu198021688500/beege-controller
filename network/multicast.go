package network

import (
	"fmt"
	"net"
)

const (
	MAX_PACKAGE_LENGTH = 2048
	UDP_MESSAGE_BUFFER = 1000
)

type MulticastHandler func(data []byte)

type MulticastServer struct {
	addressStr string
	errorCh    chan error
	messages   chan []byte
	address    *net.UDPAddr
	connection *net.UDPConn
	handlers   map[string]MulticastHandler
}

func NewMulticastServer(address string) (*MulticastServer, error) {
	srv := &MulticastServer{
		addressStr: address,
		errorCh:    make(chan error),
		messages:   make(chan []byte, UDP_MESSAGE_BUFFER),
		handlers:   make(map[string]MulticastHandler),
	}
	return srv, nil
}

func (this *MulticastServer) Run() {
	var err error
	this.address, err = net.ResolveUDPAddr("udp4", this.addressStr)
	if err != nil {
		panic(err)
	}
	this.connection, err = net.ListenMulticastUDP("udp4", nil, this.address)
	if err != nil {
		panic(err)
	}

	go this.processMessage()

	cache := make([]byte, MAX_PACKAGE_LENGTH)
	for {
		n, _, err := this.connection.ReadFromUDP(cache[0:])
		if err != nil {
			continue
		}

		data := make([]byte, n)
		copy(data[0:n], cache[0:n])

		this.messages <- data
	}

	panic("unreachable")
}

func (this *MulticastServer) processMessage() {
	for {
		message := <-this.messages
		length := len(message)
		blankIndex := length - 1

		for ; blankIndex > 0; blankIndex-- {
			if message[blankIndex] == 32 {
				break
			}
		}

		if blankIndex > 0 {
			cmd := string(message[blankIndex+1 : length])
			if handler, exists := this.handlers[cmd]; exists {
				handler(message[0:blankIndex])
			}
		}
	}
}

func (this *MulticastServer) RegisterHandler(name string, handler MulticastHandler) error {
	if _, exists := this.handlers[name]; exists {
		return fmt.Errorf("can't overwrite handler for command %s", name)
	} else {
		this.handlers[name] = handler
	}
	return nil
}

func (this *MulticastServer) MulicastMessage(b []byte) (int, error) {
	return this.connection.WriteTo(b, this.address)
}
