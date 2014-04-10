package server

import (
	"fmt"
	"os"
	"time"
)

const (
	HEARTBEAT_SECONDS    = 3
	MAX_HEARTBEAT_SECOND = 2 * HEARTBEAT_SECONDS
)

func (this *Controller) heatbeat() {
	tick := time.Tick(time.Duration(HEARTBEAT_SECONDS) * time.Second)
	for {
		select {
		case <-tick:
			this.proxyEndpointHeartbeat()
			this.internalEndpointHeartbeat()
			this.registry.CleanOfflineEndpoint(MAX_HEARTBEAT_SECOND)
		}
	}
}

func (this *Controller) internalEndpointHeartbeat() {
	hostname, _ := os.Hostname()
	data := []byte(fmt.Sprintf("%s %s %d controller_internal_heartbeat",
		this.config.InternalProtoAddr, hostname, 0))
	this.multicastServer.MulicastMessage(data)

}

func (this *Controller) proxyEndpointHeartbeat() {
	hostname, _ := os.Hostname()
	data := []byte(fmt.Sprintf("%s %s %d controller_proxy_heartbeat",
		this.config.ProxyProtoAddr, hostname, 0))
	this.multicastServer.MulicastMessage(data)
}
