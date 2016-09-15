package etcd

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	"testing"
)

func TestEtcdApi_Cluster(t *testing.T) {
	RunEtcdAPITest(func(a *EtcdApi) {
		err := a.PutCluster(api.SampleCluster())
		tools.Ok(t, err)

		c, err := a.GetCluster(api.SampleCluster().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, api.SampleCluster().Name)

		cs, err := a.ListClusters()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no clusters to list")

		errr := a.DelCluster(api.SampleCluster().Name)
		tools.Ok(t, errr)
	})
}
