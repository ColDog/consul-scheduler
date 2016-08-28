package master

import (
	"fmt"
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	"testing"
)

func setupSchedulerTest(t *testing.T, clusterName, serviceName, file string, hosts int)  {
	a := api.NewMockApi()

	for i := 0; i < hosts; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	actions.ApplyConfig(file, a)
}

func testScheduler(t *testing.T, clusterName, serviceName, file string, hosts int) {
	a := api.NewMockApi()
	s := NewDefaultScheduler(a)

	for i := 0; i < hosts; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	actions.ApplyConfig(file, a)

	cluster, err := a.GetCluster(clusterName)
	tools.Ok(t, err)

	service, err := a.GetService(serviceName)
	tools.Ok(t, err)

	tools.Ok(t, s.Schedule(cluster, service))
}

func TestDefaultScheduler_Simple(t *testing.T) {
	testScheduler(t, "default", "helloworld", "../examples/hello-world.yml", 5)
}

func TestDefaultScheduler_Complex(t *testing.T) {
	testScheduler(t, "default", "helloworld", "../examples/complex.yml", 5)
}
