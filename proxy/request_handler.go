package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RequestHandler struct {
	request  *http.Request
	response http.ResponseWriter
}

func NewRequestHandler(request *http.Request, response http.ResponseWriter) RequestHandler {
	return RequestHandler{
		request:  request,
		response: response,
	}
}

func (this *RequestHandler) HandleUnsupportedProtocol() {
	hijacker, ok := this.response.(http.Hijacker)
	if !ok {
		panic("response writer cannot hijack")
	}
	client, connection, err := hijacker.Hijack()
	if err != nil {
		body := fmt.Sprintf("%d %s: %s", http.StatusBadRequest,
			http.StatusText(http.StatusBadRequest), "Unsupported protocol.")
		http.Error(this.response, body, http.StatusBadRequest)
		return
	}
	fmt.Fprintf(connection, "HTTP/1.0 400 Bad Request\r\n\r\n")
	connection.Flush()
	client.Close()
}

func (this *RequestHandler) HandleHeartbeat() {
	this.response.WriteHeader(http.StatusOK)
	this.response.Write([]byte("ok\n"))
}

func (this *RequestHandler) HandleMissingRoute() {
	this.response.Header().Set("X-RouterError", "unknown_route")
	message := fmt.Sprintf("Requested route ('%s') does not exist.",
		this.request.Host)
	body := fmt.Sprintf("%d %s: %s", http.StatusNotFound,
		http.StatusText(http.StatusNotFound), message)
	http.Error(this.response, body, http.StatusNotFound)
}

func (this *RequestHandler) HandleTcpRequest(address string) {
	err := this.serveTcp(address)
	if err != nil {
		body := fmt.Sprintf("%d %s: %s", http.StatusBadRequest,
			http.StatusText(http.StatusBadRequest),
			"TCP forwarding to endpoint failed.")
		http.Error(this.response, body, http.StatusBadRequest)
	}
}

func (this *RequestHandler) HandleWebSocketRequest(address string) {
	this.request.URL.Scheme = "http"
	this.request.URL.Host = address

	if host, _, err := net.SplitHostPort(this.request.RemoteAddr); err == nil {
		xForwardFor := append(this.request.Header["X-Forwarded-For"], host)
		this.request.Header.Set("X-Forwarded-For", strings.Join(xForwardFor, ", "))
	}

	if _, ok := this.request.Header[http.CanonicalHeaderKey("X-Request-Start")]; !ok {
		this.request.Header.Set("X-Request-Start",
			strconv.FormatInt(time.Now().UnixNano()/1e6, 10))
	}

	err := this.serveWebSocket(address)
	if err != nil {
		body := fmt.Sprintf("%d %s: %s", http.StatusBadRequest,
			http.StatusText(http.StatusBadRequest),
			"WebSocket request to endpoint failed.")
		http.Error(this.response, body, http.StatusBadRequest)
	}
}

func (this *RequestHandler) HandleBadGateway(err error) {
	this.response.Header().Set("X-Cf-RouterError", "endpoint_failure")
	body := fmt.Sprintf("%d %s: %s", http.StatusBadGateway,
		http.StatusText(http.StatusBadGateway),
		"Registered endpoint failed to handle the request.")
	http.Error(this.response, body, http.StatusBadGateway)
}

func (this *RequestHandler) HandleHttpRequest(transport *http.Transport, address string) (*http.Response, error) {
	this.request.URL.Scheme = "http"
	this.request.URL.Host = address

	if host, _, err := net.SplitHostPort(this.request.RemoteAddr); err == nil {
		xForwardFor := append(this.request.Header["X-Forwarded-For"], host)
		this.request.Header.Set("X-Forwarded-For", strings.Join(xForwardFor, ", "))
	} else {
		log.Println("set X-Forwarded-For error:", err)
	}

	if _, ok := this.request.Header[http.CanonicalHeaderKey("X-Request-Start")]; !ok {
		this.request.Header.Set("X-Request-Start", strconv.FormatInt(time.Now().UnixNano()/1e6, 10))
	}

	this.request.Close = true
	this.request.Header.Del("Connection")

	response, err := transport.RoundTrip(this.request)
	if err != nil {
		return response, err
	}

	for k, vv := range response.Header {
		for _, v := range vv {
			this.response.Header().Add(k, v)
		}
	}

	return response, err
}

func (this *RequestHandler) WriteResponse(response *http.Response) int64 {
	this.response.WriteHeader(response.StatusCode)
	if response.Body == nil {
		return 0
	}

	var dst io.Writer = this.response
	if v, ok := this.response.(writeFlusher); ok {
		u := NewMaxLatencyWriter(v, 50*time.Millisecond)
		defer u.Stop()
		dst = u
	}
	written, err := io.Copy(dst, response.Body)
	if err != nil {
		log.Println("copy response error:", err)
	}
	return written
}

func (this *RequestHandler) serveTcp(address string) error {
	hijacker, ok := this.response.(http.Hijacker)
	if !ok {
		panic("response writer cannot hijack")
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		return err
	}

	connection, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	defer client.Close()
	defer connection.Close()

	forwardIO(client, connection)

	return nil
}

func (this *RequestHandler) serveWebSocket(address string) error {
	hijacker, ok := this.response.(http.Hijacker)
	if !ok {
		panic("response writer cannot hijack")
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		return err
	}

	connection, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	defer client.Close()
	defer connection.Close()

	if err = this.request.Write(connection); err != nil {
		return err
	}

	forwardIO(client, connection)

	return nil
}

func forwardIO(a, b net.Conn) {
	done := make(chan bool, 2)

	copy := func(dst io.Writer, src io.Reader) {
		if _, err := io.Copy(dst, src); nil != err {
			log.Println(err)
		}
		done <- true
	}

	go copy(a, b)
	go copy(b, a)

	<-done
}
