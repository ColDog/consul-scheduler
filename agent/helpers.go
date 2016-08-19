package agent

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

const (
	minTCPPort         = 0
	maxTCPPort         = 65535
	maxReservedTCPPort = 1024
	startListingPort   = 32000
	maxRandTCPPort     = maxTCPPort - (maxReservedTCPPort + 1)
)

var (
	tcpPortRand        = rand.New(rand.NewSource(time.Now().UnixNano()))
	CannotStartTaskErr = errors.New("Cannot start Task")
)

// IsTCPPortAvailable returns a flag indicating whether or not a TCP port is
// available.
func IsTCPPortAvailable(port int) bool {
	if port < minTCPPort || port > maxTCPPort {
		return false
	}
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RandomTCPPort gets a free, random TCP port between 1025-65535. If no free
// ports are available -1 is returned.
func RandomTCPPort() int {
	for i := maxReservedTCPPort; i < maxTCPPort; i++ {
		p := tcpPortRand.Intn(maxRandTCPPort) + maxReservedTCPPort + 1
		if IsTCPPortAvailable(p) {
			return p
		}
	}
	return -1
}

func AvailablePortList(count int) (res []uint) {
	for i := startListingPort; i < maxTCPPort && count > 0; i++ {
		if IsTCPPortAvailable(i) {
			count--
			res = append(res, uint(i))
		}
	}
	return res
}
