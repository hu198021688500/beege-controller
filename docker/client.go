package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/utils"
)

const userAgent = "AE2 Container Agent"

var (
	ErrInvalidEndpoint   = errors.New("Invalid endpoint")
	ErrConnectionRefused = errors.New("Cannot connect to Docker endpoint")
)

type Event struct {
	Status string
	Id     string
	From   string
	Time   int64
}

type DockerClient struct {
	endpoint    string
	endpointURL *url.URL
	client      *http.Client
	handlers    map[string]EventHandler
}

type EventHandler func(id string)

func NewDockerClient(endpoint string) (*DockerClient, error) {
	urlEndpoint, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	client := &DockerClient{
		endpoint:    endpoint,
		endpointURL: urlEndpoint,
		client:      http.DefaultClient,
		handlers:    make(map[string]EventHandler),
	}
	return client, nil
}

func (this *DockerClient) RegisterEventHandler(name string, handler EventHandler) error {
	if _, exists := this.handlers[name]; exists {
		return fmt.Errorf("can't overwrite handler for command %s", name)
	} else {
		this.handlers[name] = handler
	}
	return nil
}

func (c *DockerClient) ListenEvents() {
	var (
		resp *http.Response
	)
	for {
		log.Println("connect docker to recevie events after 3 seconds")
		time.Sleep(3 * time.Second)

		req, err := http.NewRequest("GET", "/events", bytes.NewReader(nil))
		if err != nil {
			log.Println("new http request error:", err)
			continue
		}

		req.Header.Set("User-Agent", userAgent)

		if c.endpointURL.Scheme == "unix" {
			dial, err := net.Dial(c.endpointURL.Scheme, c.endpointURL.Path)
			if err != nil {
				log.Println("connect to docker server error:", err)
				continue
			}
			clientconn := httputil.NewClientConn(dial, nil)
			resp, err = clientconn.Do(req)
			if err != nil {
				if strings.Contains(err.Error(), "connection refused") {
					log.Println("do http request error:", ErrConnectionRefused)
				}
				continue
			}
			defer clientconn.Close()
		} else {
			resp, err = c.client.Do(req)
			if err != nil {
				if strings.Contains(err.Error(), "connection refused") {
					log.Println("do http request error:", ErrConnectionRefused)
				}
				continue
			}
		}
		defer resp.Body.Close()

		data := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(data[0:])
			if err != nil {
				log.Println(err)
				continue
			}
			var event Event
			err = json.Unmarshal(data[0:n], &event)
			if err != nil {
				log.Println(err)
				continue
			}
			if hanlder, exist := c.handlers[event.Status]; exist {
				hanlder(event.Id)
			}
			log.Printf("%s", data[0:n])
		}
	}
}

func (c *DockerClient) do(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, c.getURL(path), params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", userAgent)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	var resp *http.Response
	protocol := c.endpointURL.Scheme
	address := c.endpointURL.Path
	if protocol == "unix" {
		dial, err := net.Dial(protocol, address)
		if err != nil {
			return nil, -1, err
		}
		clientconn := httputil.NewClientConn(dial, nil)
		resp, err = clientconn.Do(req)
		defer clientconn.Close()
	} else {
		resp, err = c.client.Do(req)
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, newError(resp.StatusCode, body)
	}
	return body, resp.StatusCode, nil
}

func (c *DockerClient) stream(method, path string, headers map[string]string, in io.Reader, out io.Writer) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, c.getURL(path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	for key, val := range headers {
		req.Header.Set(key, val)
	}
	var resp *http.Response
	protocol := c.endpointURL.Scheme
	address := c.endpointURL.Path
	if out == nil {
		out = ioutil.Discard
	}
	if protocol == "unix" {
		dial, err := net.Dial(protocol, address)
		if err != nil {
			return err
		}
		clientconn := httputil.NewClientConn(dial, nil)
		resp, err = clientconn.Do(req)
		defer clientconn.Close()
	} else {
		resp, err = c.client.Do(req)
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return ErrConnectionRefused
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return newError(resp.StatusCode, body)
	}
	if resp.Header.Get("Content-Type") == "application/json" {
		dec := json.NewDecoder(resp.Body)
		for {
			var m jsonMessage
			if err := dec.Decode(&m); err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			if m.Stream != "" {
				fmt.Fprintln(out, m.Stream)
			} else if m.Progress != "" {
				fmt.Fprintf(out, "%s %s\r", m.Status, m.Progress)
			} else if m.Error != "" {
				return errors.New(m.Error)
			} else {
				fmt.Fprintln(out, m.Status)
			}
		}
	} else {
		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}
	return nil
}

func (c *DockerClient) hijack(method, path string, setRawTerminal bool, in io.Reader, errStream io.Writer, out io.Writer) error {
	req, err := http.NewRequest(method, c.getURL(path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "plain/text")
	protocol := c.endpointURL.Scheme
	address := c.endpointURL.Path
	if protocol != "unix" {
		protocol = "tcp"
		address = c.endpointURL.Host
	}
	dial, err := net.Dial(protocol, address)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	clientconn.Do(req)
	defer clientconn.Close()
	rwc, br := clientconn.Hijack()
	defer rwc.Close()
	errStdout := make(chan error, 1)
	go func() {
		var err error
		if setRawTerminal {
			_, err = io.Copy(out, br)
		} else {
			_, err = utils.StdCopy(out, errStream, br)
		}
		errStdout <- err
	}()
	if inFile, ok := in.(*os.File); ok && setRawTerminal && term.IsTerminal(inFile.Fd()) && os.Getenv("NORAW") == "" {
		oldState, err := term.SetRawTerminal(inFile.Fd())
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(inFile.Fd(), oldState)
	}
	go func() {
		if in != nil {
			io.Copy(rwc, in)
		}
		if err := rwc.(interface {
			CloseWrite() error
		}).CloseWrite(); err != nil && errStream != nil {
			fmt.Fprintf(errStream, "Couldn't send EOF: %s\n", err)
		}
	}()
	if err := <-errStdout; err != nil {
		return err
	}
	return nil
}

func (c *DockerClient) getURL(path string) string {
	urlStr := strings.TrimRight(c.endpoint, "/")
	if c.endpointURL.Scheme == "unix" {
		urlStr = ""
	}
	return fmt.Sprintf("%s%s", urlStr, path)
}

type jsonMessage struct {
	Status   string `json:"status,omitempty"`
	Progress string `json:"progress,omitempty"`
	Error    string `json:"error,omitempty"`
	Stream   string `json:"stream,omitempty"`
}

func queryString(opts interface{}) string {
	if opts == nil {
		return ""
	}
	value := reflect.ValueOf(opts)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	items := url.Values(map[string][]string{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		key := field.Tag.Get("qs")
		if key == "" {
			key = strings.ToLower(field.Name)
		} else if key == "-" {
			continue
		}
		v := value.Field(i)
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				items.Add(key, "1")
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.Int() > 0 {
				items.Add(key, strconv.FormatInt(v.Int(), 10))
			}
		case reflect.Float32, reflect.Float64:
			if v.Float() > 0 {
				items.Add(key, strconv.FormatFloat(v.Float(), 'f', -1, 64))
			}
		case reflect.String:
			if v.String() != "" {
				items.Add(key, v.String())
			}
		case reflect.Ptr:
			if !v.IsNil() {
				if b, err := json.Marshal(v.Interface()); err == nil {
					items.Add(key, string(b))
				}
			}
		}
	}
	return items.Encode()
}

type Error struct {
	Status  int
	Message string
}

func newError(status int, body []byte) *Error {
	return &Error{Status: status, Message: string(body)}
}

func (e *Error) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.Status, e.Message)
}

func parseEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "unix" {
		return nil, ErrInvalidEndpoint
	}
	if u.Scheme != "unix" {
		_, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			if e, ok := err.(*net.AddrError); ok {
				if e.Err == "missing port in address" {
					return u, nil
				}
			}
			return nil, ErrInvalidEndpoint
		}
		number, err := strconv.ParseInt(port, 10, 64)
		if err == nil && number > 0 && number < 65536 {
			return u, nil
		}
	} else {
		return u, nil
	}
	return nil, ErrInvalidEndpoint
}
