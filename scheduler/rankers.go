package scheduler

import (
	"github.com/coldog/sked/api"
)

const (
	memWeight    = 8
	diskWeight   = 1
	cpuWeight    = 4
	packMaxScore = 1000000
)

func rank(hosts map[string]*api.Host, rank func(h *api.Host) int) []RankedHost {
	rs := make([]RankedHost, 0, len(hosts))

	for _, h := range hosts {
		r := RankedHost{h.Name, rank(h)}
		if len(rs) > 0 {
			if r.Score >= rs[0].Score {
				rs = append([]RankedHost{r}, rs...)
			} else if r.Score <= rs[len(rs)-1].Score {
				rs = append(rs, r)
			} else {
				for i := 0; i < len(rs)-1; i++ {
					if r.Score <= rs[i].Score && r.Score >= rs[i+1].Score {
						rs = append(rs[0:i], append([]RankedHost{r}, rs[i:]...)...)
						break
					}
				}
			}
		} else {
			rs = append(rs, r)
		}
	}

	return rs
}

// returns an int that is higher the more resources the machine has.
func SpreadRanker(hosts map[string]*api.Host) []RankedHost {
	return rank(hosts, func(h *api.Host) int {
		return score(h)
	})
}

// returns an int that is smaller the more resources the machine has.
func PackRanker(hosts map[string]*api.Host) []RankedHost {
	return rank(hosts, func(h *api.Host) int {
		return packMaxScore - score(h)
	})
}

func score(h *api.Host) int {
	return (int(h.CalculatedResources.CpuUnits) * cpuWeight) + (int(h.CalculatedResources.DiskSpace) * diskWeight) + (int(h.CalculatedResources.Memory) * memWeight)
}
