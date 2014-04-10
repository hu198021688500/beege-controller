package docker

import (
	"encoding/json"
	"net/http"
)

type RegistryImages struct {
	Id   string
	Name string
}

func (c *DockerClient) ListRegistryImages() ([]RegistryImages, error) {
	path := "/images/json"
	body, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var images []RegistryImages
	err = json.Unmarshal(body, &images)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (c *DockerClient) InspectRegistryImage(name string) (*Image, error) {
	body, status, err := c.do("GET", "/images/"+name+"/json", nil)
	if status == http.StatusNotFound {
		return nil, ErrNoSuchImage
	}
	if err != nil {
		return nil, err
	}

	var image Image
	err = json.Unmarshal(body, &image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

func (c *DockerClient) DeleteRegistryImage(name string) error {
	_, status, err := c.do("DELETE", "/repositories/"+name+"/", nil)
	if status == http.StatusNotFound {
		return ErrNoSuchImage
	}
	return err
}

func (c *DockerClient) ListRegistryImageTags(name string) (map[string]string, error) {
	body, status, err := c.do("GET", "/repositories/"+name+"/tags", nil)
	if status == http.StatusNotFound {
		return nil, ErrNoSuchImage
	}
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	err = json.Unmarshal(body, &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (c *DockerClient) AddRegistryImageTags(id, name, tag string) (string, error) {
	body, _, err := c.do("PUT", "/repositories/"+name+"/tags/"+tag, id)
	return string(body), err
}

func (c *DockerClient) RemoveRegistryImageTags(name, tag string) error {
	_, _, err := c.do("DELETE", "/repositories/"+name+"/tags/"+tag, nil)
	return err
}

func (c *DockerClient) ListRegistryImageAncestry(name string) ([]string, error) {
	body, _, err := c.do("GET", "/images/"+name+"/ancestry", nil)

	if err != nil {
		return nil, err
	}

	var ids []string
	err = json.Unmarshal(body, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
