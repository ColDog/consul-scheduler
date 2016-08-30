package master

import (
	"testing"
	"github.com/coldog/sked/api"
	"fmt"
	"math/rand"
	"time"
)

func setup() map[string]*api.Host {
	hosts := make(map[string]*api.Host)
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 10; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		h.Memory = uint64(rand.Int63n(100000))
		h.DiskSpace = uint64(rand.Int63n(100000))
		h.CpuUnits = uint64(rand.Int63n(100000))

		hosts[h.Name] = h
	}

	return hosts
}

func printRes(rs []RankedHost, hosts map[string]*api.Host) {
	for _, r := range rs {
		fmt.Printf("%+v mem: %d, cpu: %d, disk %d\n", r, hosts[r.Name].Memory, hosts[r.Name].CpuUnits, hosts[r.Name].DiskSpace)
	}
}

func TestRankers_Pack(t *testing.T) {
	hosts := setup()
	rs := PackRanker(hosts)
	printRes(rs, hosts)
}

func TestRankers_Spread(t *testing.T) {
	hosts := setup()
	rs := SpreadRanker(hosts)
	printRes(rs, hosts)
}
