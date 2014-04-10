package docker

import (
	"github.com/hugb/beege-controller/config"
)

type DockerManager struct {
	Server *DockerServer
	Client *DockerClient
}

func NewDockerManager(c *config.Config) (srv *DockerManager, err error) {
	srv = &DockerManager{}

	if srv.Server, err = NewDockerServer(c); err != nil {
		return
	}

	if srv.Client, err = NewDockerClient(c.DockerEndpoint); err != nil {
		return
	}

	return
}

func (this *DockerManager) Run() {
	go this.Client.ListenEvents()

	select {}
	//this.Server.Run()
}
