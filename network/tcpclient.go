package network

import (
	"encoding/binary"
	"net"
	"strings"
	"time"

	"github.com/hugb/beege-controller/config"
)

type TCPClient struct {
	config *config.Config
	conns  map[string]net.Conn
}

func NewTCPClient(c *config.Config) (*TCPClient, error) {
	client := &TCPClient{
		config: c,
		conns:  make(map[string]net.Conn),
	}
	return client, nil
}

func (this *TCPClient) Send(endpoint string, data []byte) (result []byte, err error) {
	var conn net.Conn
	//conn, exist := this.conns[endpoint]
	//if !exist {
	networkAndAddress := strings.SplitN(endpoint, "://", 2)
	conn, err = net.Dial(networkAndAddress[0], networkAndAddress[1])
	if err != nil {
		return
	}
	//this.conns[endpoint] = conn
	//}
	var n int
	n, err = conn.Write(this.PacketByes(data))
	if err != nil {
		return
	}

	result = make([]byte, 10)
	n, err = conn.Read(result[0:])
	if err != nil {
		return
	}
	conn.Close()
	return result[0:n], nil
}

func (this *TCPClient) PacketString(message string) []byte {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(len(message)))
	data = append(data, []byte(message)...)
	return data
}

func (this *TCPClient) PacketByes(message []byte) []byte {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(len(message)))
	data = append(data, message...)
	return data
}

func (this *TCPClient) cleanConns() {
	tick := time.Tick(time.Duration(180) * time.Second)
	for {
		select {
		case <-tick:
			for index, value := range this.conns {
				if value == nil {
					delete(this.conns, index)
				}
			}
		}
	}
}
