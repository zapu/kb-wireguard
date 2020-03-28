package libwireguard

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type HostPort struct {
	Host net.IP
	Port uint16
}

func (h HostPort) Exists() bool {
	return h.Host != nil
}

func (h HostPort) IsNil() bool {
	return h.Host == nil
}

func (h HostPort) String() string {
	return fmt.Sprintf("%s:%d", h.Host.String(), h.Port)
}

func ParseHostPort(str string) (ret HostPort) {
	arr := strings.Split(str, ":")
	if len(arr) != 2 {
		return ret
	}
	ip := net.ParseIP(arr[0])
	if ip == nil {
		return ret
	}
	v, err := strconv.ParseUint(arr[1], 10, 16)
	if err != nil {
		return ret
	}
	ret.Host = ip
	ret.Port = uint16(v)
	return ret
}
