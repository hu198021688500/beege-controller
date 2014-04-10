package registry

import (
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/hugb/beege-controller/config"
	"github.com/hugb/beege-controller/docker"
)

type Registry struct {
	sync.RWMutex

	config *config.Config

	images     map[string]*docker.APIImages
	containers map[string]*docker.APIContainers
	endpoints  map[string]*docker.Endpoint
}

func NewRegistry(c *config.Config) (*Registry, error) {
	r := &Registry{
		config:     c,
		images:     make(map[string]*docker.APIImages),
		containers: make(map[string]*docker.APIContainers),
		endpoints:  make(map[string]*docker.Endpoint),
	}
	return r, nil
}

func (this *Registry) RegisterImage(id string, image *docker.APIImages) {
	this.Lock()
	defer this.Unlock()

	log.Println("regisger image id:", id, "host:", image.Host)
	this.images[id] = image
}

func (this *Registry) UnregisterImage(id string) {
	this.Lock()
	defer this.Unlock()

	log.Println("unregister image id:", id)
	delete(this.images, id)
}

func (this *Registry) GetAllImages() []*docker.APIImages {
	this.RLock()
	defer this.RUnlock()

	var images docker.APIImagesArray
	for _, value := range this.images {
		images = append(images, value)
	}
	sort.Sort(images)
	log.Println("get all image", len(images))

	return images
}

func (this *Registry) LookupImage(id string) (*docker.APIImages, bool) {
	this.RLock()
	defer this.RUnlock()

	log.Println("lookpup image")
	image, ok := this.images[id]
	return image, ok
}

func (this *Registry) LookupByImageId(id string) string {
	log.Println("lookpup image by id")
	if image, ok := this.LookupImage(id); ok {
		return image.Host
	} else {
		return ""
	}
}

func (this *Registry) RegisterContainer(id string, container *docker.APIContainers) {
	this.Lock()
	defer this.Unlock()

	log.Println("regisger container id:", id, "host:", container.Host)
	this.containers[id] = container
	this.containers[id[0:12]] = container
}

func (this *Registry) UnregisterContainer(id string) {
	this.Lock()
	defer this.Unlock()

	log.Println("unregister container id:", id)
	delete(this.containers, id)
}

func (this *Registry) GetAllContainers() []*docker.APIContainers {
	this.RLock()
	defer this.RUnlock()

	var containers docker.APIContainersArray
	for index, value := range this.containers {
		if len(index) == 12 {
			containers = append(containers, value)
		}
	}
	sort.Sort(containers)
	log.Println("get all container", len(containers))

	return containers
}

func (this *Registry) LookupContainer(id string) (*docker.APIContainers, bool) {
	this.RLock()
	defer this.RUnlock()

	log.Println("lookpup container")
	container, ok := this.containers[id]
	return container, ok
}

func (this *Registry) LookupByContainerId(id string) string {
	log.Println("lookpup container by id")
	if id == "" {
		return ""
	}
	if container, ok := this.LookupContainer(id); ok {
		return container.Host
	} else {
		return ""
	}
}

func (this *Registry) UpdateEndpoint(address string, timestamp int64) {
	this.Lock()
	defer this.Unlock()

	if _, exist := this.endpoints[address]; exist {
		this.endpoints[address].Timestamp = timestamp
	}
}

func (this *Registry) EndpointIsExist(address string) bool {
	this.RLock()
	defer this.RUnlock()

	_, exist := this.endpoints[address]
	return exist
}

func (this *Registry) AddEndpoint(endpoint *docker.Endpoint) {
	this.Lock()
	defer this.Unlock()

	log.Printf("add endpoint[%s]\n", endpoint.Address)
	this.endpoints[endpoint.Address] = endpoint
}

func (this *Registry) DeleteEndpoint(address string) {
	this.Lock()
	defer this.Unlock()

	log.Printf("delete endpoint[%s]\n", address)
	delete(this.endpoints, address)
}

func (this *Registry) GetAllControllerProxyEndpoint() []*docker.Endpoint {
	log.Println("get all container proxy endpoint")
	return this.GetAllEndpoint(docker.CONTROLLER_PROXY_ENDPOINT)
}

func (this *Registry) GetAllDockerEndpoint() []*docker.Endpoint {
	log.Println("get all docker internal endpoint")
	return this.GetAllEndpoint(docker.DOCKER_INTERNAL_ENDPOINT)
}

func (this *Registry) GetAllAgentEndpoint() []*docker.Endpoint {
	log.Println("get all agent internal endpoint")
	return this.GetAllEndpoint(docker.AGENT_INTERNAL_ENDPOINT)
}

func (this *Registry) GetAllEndpoint(role int) []*docker.Endpoint {
	this.RLock()
	defer this.RUnlock()
	var endpoints []*docker.Endpoint
	for _, value := range this.endpoints {
		if value.Role == role {
			endpoints = append(endpoints, value)
		}
	}
	return endpoints
}

func (this *Registry) FindCantCreateContainerEndpoint() string {
	this.RLock()
	defer this.RUnlock()
	for index, value := range this.endpoints {
		if value.Role == docker.DOCKER_INTERNAL_ENDPOINT && value.Status == docker.CREATE_CONTAINER_STATUS {
			return index
		}
	}
	return ""
}

func (this *Registry) RandomOneDockeEndpoint() *docker.Endpoint {
	endpoints := this.GetAllDockerEndpoint()

	this.RLock()
	defer this.RUnlock()

	ticker := 0
	index := rand.Intn(len(endpoints))

	for _, endpoint := range endpoints {
		if ticker == index {
			return endpoint
		}
		ticker += 1
	}
	return nil
}

func (this *Registry) RandomOneAgentEndpoint() *docker.Endpoint {
	endpoints := this.GetAllAgentEndpoint()

	this.RLock()
	defer this.RUnlock()

	ticker := 0
	index := rand.Intn(len(endpoints))

	for _, endpoint := range endpoints {
		if ticker == index {
			return endpoint
		}
		ticker += 1
	}
	return nil
}

func (this *Registry) CleanOfflineEndpoint(maxInterval int64) {
	this.Lock()
	defer this.Unlock()

	now := time.Now().Unix()
	for index, value := range this.endpoints {
		if value.Timestamp+maxInterval < now {
			log.Printf("endpoint[%s] is offline\n", index)
			delete(this.endpoints, index)
		}
	}
}
