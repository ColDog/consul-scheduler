package agent

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"syscall"
	"time"
)

const (
	minTCPPort         = 0
	maxTCPPort         = 65535
	maxReservedTCPPort = 1024
	startListingPort   = 32000
	maxRandTCPPort     = maxTCPPort - (maxReservedTCPPort + 1)

	bytes    = int64(1)
	kilobyte = 1024 * bytes
	megabyte = 1024 * kilobyte
	gigabyte = 1024 * megabyte
	terabyte = 1024 * gigabyte
)

var (
	tcpPortRand        = rand.New(rand.NewSource(time.Now().UnixNano()))
	CannotStartTaskErr = errors.New("Cannot start Task")
)

// IsTCPPortAvailable returns a flag indicating whether or not a TCP port is
// available.
func IsTCPPortAvailable(port uint) bool {
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

func AvailableDiskSpace() (int64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, err
	}

	return int64(stat.Bavail * uint64(stat.Bsize)), nil
}

func ToMb(val int64) int64 {
	if val == int64(0) {
		return val
	}

	return val / megabyte
}
