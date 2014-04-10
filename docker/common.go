package docker

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	PortSpecTemplate       = "ip:hostPort:containerPort"
	PortSpecTemplateFormat = "ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort"
)

func ParsePortSpecs(ports []string) (map[Port]struct{}, map[Port][]PortBinding, error) {
	var (
		exposedPorts = make(map[Port]struct{}, len(ports))
		bindings     = make(map[Port][]PortBinding)
	)

	for _, rawPort := range ports {
		proto := "tcp"

		if i := strings.LastIndex(rawPort, "/"); i != -1 {
			proto = rawPort[i+1:]
			rawPort = rawPort[:i]
		}
		if !strings.Contains(rawPort, ":") {
			rawPort = fmt.Sprintf("::%s", rawPort)
		} else if len(strings.Split(rawPort, ":")) == 2 {
			rawPort = fmt.Sprintf(":%s", rawPort)
		}

		parts, err := PartParser(PortSpecTemplate, rawPort)
		if err != nil {
			return nil, nil, err
		}

		var (
			containerPort = parts["containerPort"]
			rawIp         = parts["ip"]
			hostPort      = parts["hostPort"]
		)

		if containerPort == "" {
			return nil, nil, fmt.Errorf("No port specified: %s<empty>", rawPort)
		}
		if _, err := strconv.ParseUint(containerPort, 10, 16); err != nil {
			return nil, nil, fmt.Errorf("Invalid containerPort: %s", containerPort)
		}
		if _, err := strconv.ParseUint(hostPort, 10, 16); hostPort != "" && err != nil {
			return nil, nil, fmt.Errorf("Invalid hostPort: %s", hostPort)
		}

		port := NewPort(proto, containerPort)
		if _, exists := exposedPorts[port]; !exists {
			exposedPorts[port] = struct{}{}
		}

		binding := PortBinding{
			HostIp:   rawIp,
			HostPort: hostPort,
		}
		bslice, exists := bindings[port]
		if !exists {
			bslice = []PortBinding{}
		}
		bindings[port] = append(bslice, binding)
	}
	return exposedPorts, bindings, nil
}

func PartParser(template, data string) (map[string]string, error) {
	// ip:public:private
	var (
		templateParts = strings.Split(template, ":")
		parts         = strings.Split(data, ":")
		out           = make(map[string]string, len(templateParts))
	)
	if len(parts) != len(templateParts) {
		return nil, fmt.Errorf("Invalid format to parse.  %s should match template %s", data, template)
	}

	for i, t := range templateParts {
		value := ""
		if len(parts) > i {
			value = parts[i]
		}
		out[t] = value
	}
	return out, nil
}
