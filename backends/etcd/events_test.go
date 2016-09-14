package etcd

import (
	"testing"
	"github.com/coldog/sked/api"
	"time"
	"fmt"
)

func TestEtcdAPI_Events(t *testing.T) {
	RunEtcdAPITest(func(a *EtcdApi) {

		go a.watch("")

		l := make(chan string)
		a.Subscribe("test", "*", l)

		c := 0
		go func() {
			for {
				e := <-l
				fmt.Println(e)
				c++
			}
		}()

		// do some stuff
		time.Sleep(1 * time.Second)
		a.PutCluster(api.SampleCluster())
		a.PutDeployment(api.SampleDeployment())
		a.DelDeployment(api.SampleDeployment().Name)
		a.PutTask(api.SampleTask())
		a.PutTaskState(api.SampleTask().ID(), api.FAILING)

		time.Sleep(1 * time.Second)

		if c < 5 {
			t.Error("count of events is not enough")
		}

	})
}
