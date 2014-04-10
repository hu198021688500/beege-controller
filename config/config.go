package config

import (
	"io/ioutil"
	"time"

	"launchpad.net/goyaml"
)

type Server struct {
}

type Config struct {
	MulticastAddr     string "multicastAddr"
	ProxyProtoAddr    string "proxyProtoAddrs"
	MonitorProtoAddr  string "monitorProtoAddrs"
	InternalProtoAddr string "internalProtoAddrs"

	TimeoutInSeconds int "Timeout"

	Timeout time.Duration
}

var defaultConfig = Config{
	MulticastAddr:     "239.255.43.99:1889",
	ProxyProtoAddr:    "tcp://192.168.1.113:9000",
	MonitorProtoAddr:  "tcp://192.168.1.113:9001",
	InternalProtoAddr: "tcp://192.168.1.113:9002",

	TimeoutInSeconds: 5,
}

func DefaultConfig() *Config {
	c := defaultConfig

	c.Process()

	return &c
}

func (c *Config) Process() {
	c.Timeout = time.Duration(c.TimeoutInSeconds) * time.Second
}

func (c *Config) Initialize(configYAML []byte) error {
	return goyaml.Unmarshal(configYAML, &c)
}

func InitConfigFromFile(path string) *Config {
	var c *Config = DefaultConfig()
	var e error

	b, e := ioutil.ReadFile(path)
	if e != nil {
		panic(e.Error())
	}

	e = c.Initialize(b)
	if e != nil {
		panic(e.Error())
	}

	c.Process()

	return c
}
