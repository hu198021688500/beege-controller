package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hugb/beege-controller/docker"
)

func (this *ProxyServer) getImagesJSON(responseWriter http.ResponseWriter, request *http.Request) error {
	host := this.getHostFromQueryParam(request)
	if host == "" {
		data, err := json.Marshal(this.Registry.GetAllImages())
		if err != nil {
			fmt.Fprintf(responseWriter, "images json encode error: %s", err)
		} else {
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.Write(data)
		}
	} else {
		this.httpProxy(host, responseWriter, request)
	}
	return nil
}

func (this *ProxyServer) getImagesSearch(responseWriter http.ResponseWriter, request *http.Request) error {
	this.proxyByHost(responseWriter, request)
	return nil
}

func (this *ProxyServer) getContainersJSON(responseWriter http.ResponseWriter, request *http.Request) error {
	host := this.getHostFromQueryParam(request)
	if host == "" {
		data, err := json.Marshal(this.Registry.GetAllContainers())
		if err != nil {
			fmt.Fprintf(responseWriter, "containers json encode error: %s", err)
		} else {
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.Write(data)
		}
	} else {
		this.httpProxy(host, responseWriter, request)
	}
	return nil
}

func (this *ProxyServer) postAuth(responseWriter http.ResponseWriter, request *http.Request) error {
	this.proxyByHost(responseWriter, request)
	return nil
}

func (this *ProxyServer) postCommit(responseWriter http.ResponseWriter, request *http.Request) error {
	var host string
	if err := request.ParseForm(); err == nil {
		containerId := request.Form.Get("container")
		if containerId != "" {
			host = this.Registry.LookupByContainerId(request.Form.Get("container"))
		} else {
			host = this.getHostFromQueryParam(request)
		}
	}
	this.httpProxy(host, responseWriter, request)
	return nil
}

func (this *ProxyServer) postBuild(responseWriter http.ResponseWriter, request *http.Request) error {
	this.proxyByHost(responseWriter, request)
	return nil
}

func (this *ProxyServer) postImagesLoad(responseWriter http.ResponseWriter, request *http.Request) error {
	this.proxyByHost(responseWriter, request)
	return nil
}

func (this *ProxyServer) postContainersCreate(responseWriter http.ResponseWriter, request *http.Request) error {
	this.httpProxy(this.Registry.FindCantCreateContainerEndpoint(), responseWriter, request)
	return nil
}

func (this *ProxyServer) proxyWithImageId(responseWriter http.ResponseWriter, request *http.Request) error {
	this.httpProxy(this.LookupByImageId(this.getIdFromPath(request)), responseWriter, request)
	return nil
}

func (this *ProxyServer) proxyWithContainerId(responseWriter http.ResponseWriter, request *http.Request) error {
	this.httpProxy(this.Registry.LookupByContainerId(this.getIdFromPath(request)), responseWriter, request)
	return nil
}

func (this *ProxyServer) getAllAgent(responseWriter http.ResponseWriter, request *http.Request) error {
	data, err := json.Marshal(this.Registry.GetAllAgentEndpoint())
	if err != nil {
		fmt.Fprintf(responseWriter, "agents json encode error: %s", err)
	} else {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write(data)
	}
	return nil
}

func (this *ProxyServer) getAllDocker(responseWriter http.ResponseWriter, request *http.Request) error {
	data, err := json.Marshal(this.Registry.GetAllDockerEndpoint())
	if err != nil {
		fmt.Fprintf(responseWriter, "dockers json encode error: %s", err)
	} else {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write(data)
	}
	return nil
}

func (this *ProxyServer) getAllController(responseWriter http.ResponseWriter, request *http.Request) error {
	data, err := json.Marshal(this.Registry.GetAllControllerProxyEndpoint())
	if err != nil {
		fmt.Fprintf(responseWriter, "controller json encode error: %s", err)
	} else {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write(data)
	}
	return nil
}

type vm struct {
	instanceId string                               `json:"instance_id"`
	imageId    string                               `json:"image_id"`
	status     string                               `json:"status"`
	flavor     map[string]string                    `json:"flavor"`
	host       string                               `json:"host"`
	created    string                               `json:"created"`
	nics       map[docker.Port][]docker.PortBinding `json:"nics"`
}

func (this *ProxyServer) createVm(responseWriter http.ResponseWriter, request *http.Request) error {
	// get post form data
	params := &KeyValue{}
	if err := params.Decode(request.Body); err != nil {
		return err
	}
	// check params
	imageId := params.Get("image_id")
	if imageId == "" {
		return errors.New("image_id is required.")
	}

	go func() {
		// docker client init
		dockerEndpoint := this.Registry.GetAllDockerEndpoint()
		dockerClient, err := docker.NewDockerClient(dockerEndpoint[0].Address)

		host := this.Registry.LookupByImageId(imageId)
		if host == "" {
			// todo:镜像id和name转化
			log.Println("Pull image", imageId)
			opts := docker.PullImageOptions{Repository: "10.0.0.27/" + imageId}
			auth := docker.AuthConfiguration{}
			if err := dockerClient.PullImage(opts, auth); err != nil {

			}
		}
		// sleep and retry lookup host
		host = this.Registry.LookupByImageId(imageId)
		opts := docker.CreateContainerOptions{}
		container, err := dockerClient.CreateContainer(opts)
		if err != nil {

		}

		if err = dockerClient.StartContainer(container.ID, &docker.HostConfig{}); err != nil {

		}
		result := struct {
			requestId string `json:"request_id"`
			code      string `json:"code"`
			message   string `json:"message"`
			data      []vm   `json:"data"`
		}{}
		log.Println(result)
		// send asynchronous message to rabbitmq
	}()

	var out KeyValue
	out.Set("request_id", docker.GenerateUUID())
	out.Set("code", "0")
	out.Set("message", "")
	writeJSON(responseWriter, http.StatusCreated, out)

	return nil
}

// 输出json
func writeJSON(w http.ResponseWriter, code int, v KeyValue) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return v.Encode(w)
}

// 删除虚拟机
func (this *ProxyServer) deleteVms(responseWriter http.ResponseWriter, request *http.Request) error {
	if err := request.ParseForm(); err != nil {
		return err
	}
	id := strings.TrimSpace(request.Form.Get("id"))
	if id == "" {
		return errors.New("id is required.")
	}
	ids := strings.Split(id, ",")
	if len(ids) == 1 {
		request.URL.Path = "/containers/" + ids[0]
		this.proxyWithContainerId(responseWriter, request)
	} else {
		return errors.New("Delete does not support multiple virtual machines.")
	}

	return nil
}
