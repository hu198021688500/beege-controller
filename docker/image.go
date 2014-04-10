package docker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

// This work with api verion < v1.7 and > v1.9
type APIImages struct {
	ID string `json:"Id"`
	//Comment     string   `json:"comment,omitempty"`
	//Container   string   `json:"container,omitempty"`
	RepoTags    []string `json:",omitempty"`
	Created     int64
	Size        uint64
	VirtualSize uint64
	ParentId    string `json:",omitempty"`
	//Repository  string `json:",omitempty"`
	//Tag         string `json:",omitempty"`
	Host string `json:",omitempty"`
}

type APIImagesArray []*APIImages

func (this APIImagesArray) Len() int {
	return len(this)
}

func (this APIImagesArray) Less(i, j int) bool {
	return this[i].Created > this[j].Created
}

func (this APIImagesArray) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

// Error returned when the image does not exist.
var (
	ErrNoSuchImage         = errors.New("No such image")
	ErrMissingRepo         = errors.New("Missing remote repository e.g. 'github.com/user/repo'")
	ErrMissingOutputStream = errors.New("Missing output stream")
)

// ListImages returns the list of available images in the server.
//
// See http://docs.docker.io/en/latest/reference/api/docker_remote_api_v1.9/#list-images for more details.
func (c *DockerClient) ListImages(all bool) ([]APIImages, error) {
	path := "/images/json?all="
	if all {
		path += "1"
	} else {
		path += "0"
	}
	body, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var images []APIImages
	err = json.Unmarshal(body, &images)
	if err != nil {
		return nil, err
	}
	return images, nil
}

// RemoveImage removes an image by its name or ID.
//
// See http://goo.gl/7hjHHy for more details.
func (c *DockerClient) RemoveImage(name string) error {
	_, status, err := c.do("DELETE", "/images/"+name, nil)
	if status == http.StatusNotFound {
		return ErrNoSuchImage
	}
	return err
}

// InspectImage returns an image by its name or ID.
//
// See http://goo.gl/pHEbma for more details.
func (c *DockerClient) InspectImage(name string) (*Image, error) {
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

// PushImageOptions represents options to use in the PushImage method.
//
// See http://goo.gl/GBmyhc for more details.
type PushImageOptions struct {
	// Name of the image
	Name string

	// Registry server to push the image
	Registry string

	OutputStream io.Writer `qs:"-"`
}

// AuthConfiguration represents authentication options to use in the PushImage
// method. It represents the authencation in the Docker index server.
type AuthConfiguration struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
}

// PushImage pushes an image to a remote registry, logging progress to w.
//
// An empty instance of AuthConfiguration may be used for unauthenticated
// pushes.
//
// See http://goo.gl/GBmyhc for more details.
func (c *DockerClient) PushImage(opts PushImageOptions, auth AuthConfiguration) error {
	if opts.Name == "" {
		return ErrNoSuchImage
	}
	name := opts.Name
	opts.Name = ""
	path := "/images/" + name + "/push?" + queryString(&opts)
	var headers = make(map[string]string)
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(auth)

	headers["X-Registry-Auth"] = base64.URLEncoding.EncodeToString(buf.Bytes())

	return c.stream("POST", path, headers, nil, opts.OutputStream)
}

// PullImageOptions present the set of options available for pulling an image
// from a registry.
//
// See http://goo.gl/PhBKnS for more details.
type PullImageOptions struct {
	Repository   string `qs:"fromImage"`
	Registry     string
	OutputStream io.Writer `qs:"-"`
}

// PullImage pulls an image from a remote registry, logging progress to w.
//
// See http://goo.gl/PhBKnS for more details.
func (c *DockerClient) PullImage(opts PullImageOptions, auth AuthConfiguration) error {
	if opts.Repository == "" {
		return ErrNoSuchImage
	}

	var headers = make(map[string]string)
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(auth)
	headers["X-Registry-Auth"] = base64.URLEncoding.EncodeToString(buf.Bytes())

	return c.createImage(queryString(&opts), headers, nil, opts.OutputStream)
}

func (c *DockerClient) createImage(qs string, headers map[string]string, in io.Reader, w io.Writer) error {
	path := "/images/create?" + qs
	return c.stream("POST", path, headers, in, w)
}

// ImportImageOptions present the set of informations available for importing
// an image from a source file or the stdin.
//
// See http://goo.gl/PhBKnS for more details.
type ImportImageOptions struct {
	Repository string `qs:"repo"`
	Source     string `qs:"fromSrc"`

	InputStream  io.Reader `qs:"-"`
	OutputStream io.Writer `qs:"-"`
}

// ImportImage imports an image from a url, a file or stdin
//
// See http://goo.gl/PhBKnS for more details.
func (c *DockerClient) ImportImage(opts ImportImageOptions) error {
	if opts.Repository == "" {
		return ErrNoSuchImage
	}
	if opts.Source != "-" {
		opts.InputStream = nil
	}
	if opts.Source != "-" && !isUrl(opts.Source) {
		f, err := os.Open(opts.Source)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadAll(f)
		opts.InputStream = bytes.NewBuffer(b)
		opts.Source = "-"
	}
	return c.createImage(queryString(&opts), nil, opts.InputStream, opts.OutputStream)
}

// BuildImageOptions present the set of informations available for building
// an image from a tarfile with a Dockerfile in it,the details about Dockerfile
// see http://docs.docker.io/en/latest/reference/builder/
type BuildImageOptions struct {
	Name           string    `qs:"t"`
	NoCache        bool      `qs:"nocache"`
	SuppressOutput bool      `qs:"q"`
	RmTmpContainer bool      `qs:"rm"`
	InputStream    io.Reader `qs:"-"`
	OutputStream   io.Writer `qs:"-"`
	Remote         string    `qs:"remote"`
}

// BuildImage builds an image from a tarball's url or a Dockerfile in the input
// stream.
func (c *DockerClient) BuildImage(opts BuildImageOptions) error {
	if opts.OutputStream == nil {
		return ErrMissingOutputStream
	}
	var headers map[string]string
	if opts.Remote != "" && opts.Name == "" {
		opts.Name = opts.Remote
	}
	if opts.InputStream != nil {
		headers = map[string]string{"Content-Type": "application/tar"}
	} else if opts.Remote == "" {
		return ErrMissingRepo
	}
	return c.stream("POST", fmt.Sprintf("/build?%s",
		queryString(&opts)), headers, opts.InputStream, opts.OutputStream)
}

type ImageInsertFileOptions struct {
	Path string `qs:"path"`
	Url  string `qs:"url"`
}

func (c *DockerClient) ImageInsertFile(name string, opts ImageInsertFileOptions) error {
	_, _, err := c.do("POST", fmt.Sprintf("/images/%s/insert?%s", name, queryString(&opts)), nil)
	return err
}

type ImageHistory struct {
	Id        string
	Created   int64
	CreatedBy string
}

func (c *DockerClient) GetImageHistory(name string) ([]ImageHistory, error) {
	body, _, err := c.do("GET", "/images/"+name+"/history", nil)
	if err != nil {
		return nil, err
	}
	var history []ImageHistory
	err = json.Unmarshal(body, &history)
	if err != nil {
		return nil, err
	}
	return history, nil
}

type ImageTagOptions struct {
	Repo  string
	Force bool
}

func (c *DockerClient) TagImage(name string, opts ImageTagOptions) error {
	_, _, err := c.do("POST", fmt.Sprintf("/images/%s/tag?%s", name, queryString(&opts)), nil)
	if err != nil {
		return err
	}

	return nil
}

func isUrl(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	return p.Scheme == "http" || p.Scheme == "https"
}
