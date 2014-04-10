package server

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hugb/beege-controller/docker"
	"github.com/hugb/beege-controller/network"
)

func (this *Controller) multicastHandlers() {
	m := map[string]network.MulticastHandler{
		"agent_internal_heartbeat":      this.AgentInternalHeartbeat,
		"docker_internal_heartbeat":     this.DockerInternalHeartbeat,
		"controller_proxy_heartbeat":    this.ControllerProxyHeartbeat,
		"controller_internal_heartbeat": this.ControllerInternalHeartbeat,
	}
	for cmd, fct := range m {
		if err := this.multicastServer.RegisterHandler(cmd, fct); err != nil {
			log.Printf("register multicast handler[%s] failure:%s\n", cmd, err)
		} else {
			log.Printf("register multicast handler[%s] success\n", cmd)
		}
	}
}

func (this *Controller) AgentInternalHeartbeat(data []byte) {
	this.heartbeat(docker.AGENT_INTERNAL_ENDPOINT, string(data))
}

func (this *Controller) DockerInternalHeartbeat(data []byte) {
	this.heartbeat(docker.DOCKER_INTERNAL_ENDPOINT, string(data))
}

func (this *Controller) ControllerProxyHeartbeat(data []byte) {
	this.heartbeat(docker.CONTROLLER_PROXY_ENDPOINT, string(data))
}

func (this *Controller) ControllerInternalHeartbeat(data []byte) {
	this.heartbeat(docker.CONTROLLER_INTERNAL_ENDPOINT, string(data))
}

func (this *Controller) heartbeat(role int, data string) {
	endpointParts := strings.SplitN(data, " ", 3)
	if !this.registry.EndpointIsExist(endpointParts[0]) {
		if status, err := strconv.Atoi(endpointParts[2]); err == nil {
			host := &docker.Endpoint{
				Address:   endpointParts[0],
				Hostname:  endpointParts[1],
				Status:    status,
				Role:      role,
				Timestamp: time.Now().Unix(),
			}
			this.registry.AddEndpoint(host)
		} else {
			log.Println("heartbeat packet status convert failure")
		}
	} else {
		this.registry.UpdateEndpoint(endpointParts[0], time.Now().Unix())
	}
}

func (this *Controller) addMyselfEndpoint() {
	hostname, _ := os.Hostname()
	host := &docker.Endpoint{
		Address:   this.config.InternalProtoAddr,
		Hostname:  hostname,
		Status:    1,
		Role:      docker.CONTROLLER_INTERNAL_ENDPOINT,
		Timestamp: 1<<63 - MAX_HEARTBEAT_SECOND, // = 2^64 - 1,
	}
	this.registry.AddEndpoint(host)

	host1 := &docker.Endpoint{
		Address:   this.config.ProxyProtoAddr,
		Hostname:  hostname,
		Status:    1,
		Role:      docker.CONTROLLER_PROXY_ENDPOINT,
		Timestamp: 1<<63 - MAX_HEARTBEAT_SECOND,
	}
	this.registry.AddEndpoint(host1)
}

func (this *Controller) tcpHandlers() {
	m := map[string]network.TcpHandler{
		"report_image_list":        this.Images,
		"report_image_created":     this.ImageCreated,
		"report_image_updated":     this.ImageUpdated,
		"report_image_deleted":     this.ImageDeleted,
		"report_container_list":    this.Containers,
		"report_container_created": this.ContainerCreated,
		"report_container_updated": this.ContainerUpdated,
		"report_container_deleted": this.ContainerDeleted,
	}
	for cmd, fct := range m {
		if err := this.tcpServer.RegisterHandler(cmd, fct); err != nil {
			log.Printf("register tcp handler[%s] failure:%s\n", cmd, err)
		} else {
			log.Printf("register tcp hander[%s] success\n", cmd)
		}
	}
}

func (this *Controller) Images(data []byte) error {
	var images []docker.APIImages
	if err := json.Unmarshal(data, &images); err != nil {
		log.Println("images decode error:", err)
		return err
	}
	for index, value := range images {
		//不能使用&value而要使用&images[index]，使用&value会得到同一个内存地址
		this.registry.RegisterImage(value.ID, &images[index])
	}
	return nil
}

func (this *Controller) ImageCreated(data []byte) error {
	var image docker.APIImages
	if err := json.Unmarshal(data, &image); err != nil {
		log.Println("image decode error:", err)
		return err
	}
	this.registry.RegisterImage(image.ID, &image)
	return nil
}

func (this *Controller) ImageUpdated(data []byte) error {
	var image docker.APIImages
	if err := json.Unmarshal(data, &image); err != nil {
		log.Println("image decode error:", err)
		return err
	} else {
		this.registry.RegisterImage(image.ID, &image)
		return nil
	}
}

func (this *Controller) ImageDeleted(data []byte) error {
	var image docker.APIImages
	if err := json.Unmarshal(data, &image); err != nil {
		log.Println("image decode error:", err)
		return err
	}
	this.registry.UnregisterImage(image.ID)
	return nil
}

func (this *Controller) Containers(data []byte) error {
	var containers []docker.APIContainers
	if err := json.Unmarshal(data, &containers); err != nil {
		log.Println("containers decode error:", err)
		return err
	}
	for index, value := range containers {
		this.registry.RegisterContainer(value.ID, &containers[index])
	}
	return nil
}

func (this *Controller) ContainerCreated(data []byte) error {
	var container docker.APIContainers

	if err := json.Unmarshal(data, &container); err != nil {
		log.Println("container decode error:", err)
		return err
	}
	this.registry.RegisterContainer(container.ID, &container)
	return nil
}

func (this *Controller) ContainerUpdated(data []byte) error {
	var container docker.APIContainers
	if err := json.Unmarshal(data, &container); err != nil {
		log.Println("container decode error:", err)
		return err
	}
	this.registry.RegisterContainer(container.ID, &container)
	return nil
}

func (this *Controller) ContainerDeleted(data []byte) error {
	this.registry.UnregisterContainer(string(data))
	return nil
}
