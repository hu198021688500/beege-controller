package docker

import (
	"encoding/json"
	"log"
)

type DockerInfo struct {
	Containers         int
	Images             int
	Debug              int
	Driver             string
	DriverStatus       [][]string
	ExecutionDriver    string
	IPv4Forwarding     int
	IndexServerAddress string
	InitPath           string
	InitSha1           string
	KernelVersion      string
	MemoryLimit        int
	NEventsListener    int
	NFd                int
	NGoroutines        int
	SwapLimit          int
}

func (c *DockerClient) Build() {

}

func (c *DockerClient) Auth() {

}

func (c *DockerClient) Info() (*DockerInfo, error) {
	body, _, err := c.do("GET", "/info", nil)
	if err != nil {
		return nil, err
	}
	var info DockerInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *DockerClient) Endpoints(role string) ([]Endpoint, error) {
	//body, _, err := c.do("GET", "/"+role+"/json", nil)
	body, _, err := c.do("GET", "/manage/host", nil)
	if err != nil {
		return nil, err
	}
	var hosts []string
	err = json.Unmarshal(body, &hosts)
	if err != nil {
		return nil, err
	}
	var endpoints []Endpoint
	for _, value := range hosts {
		endpoints = append(endpoints, Endpoint{Address: value})
	}
	return endpoints, nil
}

func (c *DockerClient) GetDockerHosts() ([]string, error) {
	body, _, err := c.do("GET", "/manage/host", nil)
	if err != nil {
		return nil, err
	}
	var hosts []string
	err = json.Unmarshal(body, &hosts)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

type HostRes struct {
	Cpu         string `json:"Cpu"`
	Memory      string `json:"Memory"`
	Loadaverage string `json:"Loadaverage"`
}

type HostInfo struct {
	Status int     `json:"Status"`
	Res    HostRes `json:"Res"`
}

func (c *DockerClient) GetHost(name string) (*HostInfo, error) {
	body, _, err := c.do("GET", "/manage/host/"+name+"/res", nil)
	if err != nil {
		return nil, err
	}
	log.Println(string(body))
	host := &HostInfo{}
	err = json.Unmarshal(body, host)
	if err != nil {
		return nil, err
	}
	log.Println(host)
	return host, nil
}

func (c *DockerClient) Pull(name string) error {
	_, _, err := c.do("GET", "/images/"+name+"/pull", nil)
	if err != nil {
		return err
	}
	return nil
}

type DockerVersion struct {
	Arch          string
	GitCommit     interface{}
	GoVersion     string
	KernelVersion string
	Os            string
	Version       string
}

func (c *DockerClient) Version() (*DockerVersion, error) {
	body, _, err := c.do("GET", "/version", nil)
	if err != nil {
		return nil, err
	}
	var version DockerVersion
	err = json.Unmarshal(body, &version)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (c *DockerClient) Commit() {

}

func (c *DockerClient) Events() {

}

func (c *DockerClient) GetImages() {

}

func (c *DockerClient) LoadImages() {

}
