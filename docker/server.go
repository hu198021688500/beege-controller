package docker

import (
	"fmt"
	"io"
	//"log"
	"os"
	"os/exec"
	"syscall"
	//"time"

	"github.com/kr/pty"

	"github.com/hugb/beege-controller/config"
)

const (
	AGENT_INTERNAL_ENDPOINT = iota
	DOCKER_INTERNAL_ENDPOINT
	CONTROLLER_INTERNAL_ENDPOINT
	CONTROLLER_PROXY_ENDPOINT
	CREATE_CONTAINER_STATUS
)

type Endpoint struct {
	Address   string
	Hostname  string
	Role      int
	Status    int
	Timestamp int64
}

type HostStatus struct {
	CpuUsage    float64
	LoadAverage float64
	MemFree     uint64
	SwapFree    uint64
}

type DockerServer struct {
	run    bool
	cmd    *exec.Cmd
	config *config.Config
}

func NewDockerServer(c *config.Config) (*DockerServer, error) {
	this := &DockerServer{
		config: c,
	}
	if this.config.DockerExePath == "" {
		this.config.DockerExePath = os.Getenv("DOCKER_EXE_PATH")
	}
	if this.config.DockerExePath == "" {
		if path, err := exec.LookPath("docker"); err == nil {
			this.config.DockerExePath = path
		} else {
			panic(err)
		}
	}
	params := []string{this.config.DockerExePath}
	params = append(params, "-d")
	params = append(params, "-H")
	params = append(params, this.config.DockerEndpoint)
	params = append(params, "-H")
	params = append(params, fmt.Sprintf("tcp://%s", this.config.DockerHost))

	this.cmd = exec.Command(params[0], params[1:]...)

	return this, nil
}

func (this *DockerServer) Run() {
	f, err := pty.Start(this.cmd)
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, f)
}

func (this *DockerServer) Stop() error {
	return this.cmd.Process.Signal(syscall.SIGQUIT)
}

func (this *DockerServer) ReStart() error {
	return nil
}

func (this *DockerServer) IsRunning() (bool, error) {
	return true, nil
}

func GetDockerHostStatus() int {
	return CREATE_CONTAINER_STATUS
}
