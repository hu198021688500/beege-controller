package proxy

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/hugb/beege-controller/config"
	"github.com/hugb/beege-controller/registry"
)

const (
	API_VERSION = "0.1"
)

type HttpApiFunc func(w http.ResponseWriter, r *http.Request) error

type ProxyServer struct {
	*config.Config
	*registry.Registry
	*http.Transport
}

func NewProxyServer(c *config.Config, r *registry.Registry) (*ProxyServer, error) {
	srv := &ProxyServer{
		Config:    c,
		Registry:  r,
		Transport: &http.Transport{ResponseHeaderTimeout: c.Timeout},
	}
	return srv, nil
}

// listen
func (this *ProxyServer) Run() {
	route, err := this.createRouter()
	if err != nil {
		panic(err)
	}

	protoAddrParts := strings.SplitN(this.Config.ProxyProtoAddr, "://", 2)

	ln, err := net.Listen(protoAddrParts[0], protoAddrParts[1])
	if err != nil {
		panic(err)
	}

	httpSrv := http.Server{Addr: protoAddrParts[1], Handler: route}
	if err = httpSrv.Serve(ln); err != nil {
		panic(err)
	}
}

func (this *ProxyServer) createRouter() (*mux.Router, error) {
	r := mux.NewRouter()
	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/events":                         this.proxyByHost,
			"/info":                           this.proxyRondomOrByHost,
			"/version":                        this.proxyRondomOrByHost,
			"/images/json":                    this.getImagesJSON,
			"/images/search":                  this.getImagesSearch,
			"/images/{name:.*}/get":           this.proxyWithImageId,
			"/images/{name:.*}/history":       this.proxyWithImageId,
			"/images/{name:.*}/json":          this.proxyWithImageId,
			"/containers/ps":                  this.getContainersJSON,
			"/containers/json":                this.getContainersJSON,
			"/containers/{name:.*}/export":    this.proxyWithContainerId,
			"/containers/{name:.*}/changes":   this.proxyWithContainerId,
			"/containers/{name:.*}/json":      this.proxyWithContainerId,
			"/containers/{name:.*}/top":       this.proxyWithContainerId,
			"/containers/{name:.*}/attach/ws": this.proxyWithContainerId,

			"/controllers/json": this.getAllController,
			"/dockers/json":     this.getAllDocker,
			"/agents/json":      this.getAllAgent,
		},
		"POST": {
			"/auth":                         this.postAuth,
			"/commit":                       this.postCommit,
			"/build":                        this.postBuild,
			"/images/create":                this.proxyRondomOrByHost,
			"/images/{name:.*}/insert":      this.proxyWithImageId,
			"/images/load":                  this.postImagesLoad,
			"/images/{name:.*}/push":        this.proxyWithImageId,
			"/images/{name:.*}/tag":         this.proxyWithImageId,
			"/containers/create":            this.postContainersCreate,
			"/containers/{name:.*}/kill":    this.proxyWithContainerId,
			"/containers/{name:.*}/restart": this.proxyWithContainerId,
			"/containers/{name:.*}/start":   this.proxyWithContainerId,
			"/containers/{name:.*}/stop":    this.proxyWithContainerId,
			"/containers/{name:.*}/wait":    this.proxyWithContainerId,
			"/containers/{name:.*}/resize":  this.proxyWithContainerId,
			"/containers/{name:.*}/attach":  this.proxyWithContainerId,
			"/containers/{name:.*}/copy":    this.proxyWithContainerId,

			"/server":                 this.createVm,
			"/server/{name:.*}/start": this.proxyWithContainerId,
			"/server/{name:.*}/stop":  this.proxyWithContainerId,
		},
		"DELETE": {
			"/containers/{name:.*}": this.proxyWithContainerId,
			"/images/{name:.*}":     this.proxyWithImageId,

			"/server": this.deleteVms,
		},
	}
	for method, routes := range m {
		for route, fct := range routes {
			log.Printf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localFct := fct
			localRoute := route
			localMethod := method

			// build the handler function
			f := makeHttpHandler(localFct)

			// add the new route
			if localRoute == "" {
				r.Methods(localMethod).HandlerFunc(f)
			} else {
				r.Path(localRoute).Methods(localMethod).HandlerFunc(f)
				r.Path("/v{version:[0-9.]+}" + localRoute).Methods(localMethod).HandlerFunc(f)
			}
		}
	}

	return r, nil
}

func makeHttpHandler(handlerFunc HttpApiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// todo:验证版本兼容性

		// todo:处理所有api的公共业务逻辑

		if err := handlerFunc(w, r); err != nil {
			httpError(w, err)
		}
	}
}

// 根据错误生成不同的http错误响应
//
func httpError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	statusCode := http.StatusInternalServerError
	if strings.Contains(err.Error(), "No such") {
		statusCode = http.StatusNotFound
	} else if strings.Contains(err.Error(), "Bad parameter") {
		statusCode = http.StatusBadRequest
	} else if strings.Contains(err.Error(), "Conflict") {
		statusCode = http.StatusConflict
	} else if strings.Contains(err.Error(), "Impossible") {
		statusCode = http.StatusNotAcceptable
	} else if strings.Contains(err.Error(), "Wrong login/password") {
		statusCode = http.StatusUnauthorized
	} else if strings.Contains(err.Error(), "hasn't been activated") {
		statusCode = http.StatusForbidden
	} else {
		//http.StatusInternalServerError
	}

	http.Error(w, err.Error(), statusCode)
}

// 获取路径中name
func (this *ProxyServer) getIdFromPath(request *http.Request) string {
	if request == nil {
		return ""
	}
	vars := mux.Vars(request)
	if vars == nil {
		return ""
	} else {
		return vars["name"]
	}
}

// 去掉endpoint中的protocol
func (this *ProxyServer) RandomOneDockeHost() (host string) {
	endpoint := this.Registry.RandomOneDockeEndpoint()
	if endpoint != nil {
		if strings.Contains(endpoint.Address, "://") {
			endpointParts := strings.SplitN(endpoint.Address, "://", 2)
			host = endpointParts[1]
		} else {
			host = endpoint.Address
		}
	}
	return
}

// 获取querystring中的host
func (this *ProxyServer) getHostFromQueryParam(request *http.Request) string {
	if request == nil {
		return ""
	}
	if err := request.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return ""
	} else {
		return request.Form.Get("host")
	}
}

// 在querystring中指定后端服务器的地址
// for example http://127.0.0.1:80/v1.0/events?host=127.0.0.1:80
func (this *ProxyServer) proxyByHost(responseWriter http.ResponseWriter, request *http.Request) error {
	this.httpProxy(this.getHostFromQueryParam(request), responseWriter, request)
	return nil
}

// 将请求随机地分配到后端服务器上
func (this *ProxyServer) proxyRandomHost(responseWriter http.ResponseWriter, request *http.Request) error {
	this.httpProxy(this.RandomOneDockeHost(), responseWriter, request)
	return nil
}

// 如果没有指定host则随机分配一个
func (this *ProxyServer) proxyRondomOrByHost(responseWriter http.ResponseWriter, request *http.Request) error {
	host := this.getHostFromQueryParam(request)
	if host == "" {
		this.proxyRandomHost(responseWriter, request)
	} else {
		this.httpProxy(host, responseWriter, request)
	}
	return nil
}

// 代理到后端web服务器
func (this *ProxyServer) httpProxy(host string, responseWriter http.ResponseWriter, request *http.Request) {
	handler := NewRequestHandler(request, responseWriter)
	// 仅支持http1.0和1.1
	if !isProtocolSupported(request) {
		handler.HandleUnsupportedProtocol()
		return
	}
	// 负载均衡健康检查
	if isLoadBalancerHeartbeat(request) {
		handler.HandleHeartbeat()
		return
	}
	if host == "" {
		handler.HandleMissingRoute()
		return
	}
	if isTcpUpgrade(request) {
		handler.HandleTcpRequest(host)
		return
	}
	// websocket代理支持
	if isWebSocketUpgrade(request) {
		handler.HandleWebSocketRequest(host)
		return
	}
	response, err := handler.HandleHttpRequest(this.Transport, host)
	if err != nil {
		handler.HandleBadGateway(err)
		return
	}
	handler.WriteResponse(response)
}

func isProtocolSupported(request *http.Request) bool {
	return request.ProtoMajor == 1 && (request.ProtoMinor == 0 || request.ProtoMinor == 1)
}

func isLoadBalancerHeartbeat(request *http.Request) bool {
	return request.UserAgent() == "HTTP-Monitor/1.1"
}

func isTcpUpgrade(request *http.Request) bool {
	return upgradeHeader(request) == "tcp"
}

func isWebSocketUpgrade(request *http.Request) bool {
	return strings.ToLower(upgradeHeader(request)) == "websocket"
}

func upgradeHeader(request *http.Request) string {
	if strings.ToLower(request.Header.Get("Connection")) == "upgrade" {
		return request.Header.Get("Upgrade")
	} else {
		return ""
	}
}
