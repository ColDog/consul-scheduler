package etcd

import (
	"testing"
	"github.com/coldog/sked/tools"
)

func TestEtcdApi_Locking(t *testing.T) {
	RunEtcdAPITest(func(a *EtcdApi) {
		lock, err := a.Lock("test")
		tools.Ok(t, err)

		_, err = lock.Lock()
		tools.Ok(t, err)

		tools.Assert(t, lock.IsHeld(), "not held")

		_, err = lock.Lock()
		tools.Assert(t, err != nil, err.Error())
	})
}
