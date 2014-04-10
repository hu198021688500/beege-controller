package server

import (
	"runtime"

	"github.com/hugb/beege-controller/config"
	"github.com/hugb/beege-controller/network"
	"github.com/hugb/beege-controller/proxy"
	"github.com/hugb/beege-controller/registry"
)

type Controller struct {
	config          *config.Config
	tcpServer       *network.TCPServer
	tcpClient       *network.TCPClient
	registry        *registry.Registry
	proxyServer     *proxy.ProxyServer
	multicastServer *network.MulticastServer
}

func NewController(c *config.Config) *Controller {
	controller := &Controller{
		config: c,
	}

	var err error
	runtime.GOMAXPROCS(runtime.NumCPU())

	controller.tcpServer, err = network.NewTCPServer(c.InternalProtoAddr)
	if err != nil {
		panic("init tcp server faild.")
	}

	controller.registry, err = registry.NewRegistry(c)
	if err != nil {
		panic("init registry faild.")
	}

	controller.proxyServer, err = proxy.NewProxyServer(c, controller.registry)
	if err != nil {
		panic("init proxy server faild.")
	}

	controller.multicastServer, err = network.NewMulticastServer(c.MulticastAddr)
	if err != nil {
		panic("init multicast server faild.")
	}

	controller.tcpHandlers()
	controller.multicastHandlers()
	controller.addMyselfEndpoint()

	return controller
}

func (this *Controller) Start() {
	go this.tcpServer.Run()

	go this.proxyServer.Run()

	go this.multicastServer.Run()

	this.heatbeat()
}
