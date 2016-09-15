package etcd

import (
	"github.com/coldog/sked/tools"
	"testing"
	"time"
)

func TestEtcdApi_Locking(t *testing.T) {
	RunEtcdAPITest(func(a *EtcdApi) {
		lock, err := a.Lock("test")
		tools.Ok(t, err)

		_, err = lock.Lock()
		tools.Ok(t, err)

		tools.Assert(t, lock.IsHeld(), "not held")

		_, err = lock.Lock()
		tools.Ok(t, err)

		lock2, err := a.Lock("test")
		tools.Ok(t, err)

		_, err = lock2.Lock()

		tools.Assert(t, !lock2.IsHeld(), "lock is held")
	})
}

func TestEtcdApi_LockingRefresh(t *testing.T) {
	RunEtcdAPITest(func(a *EtcdApi) {
		a.config.LockTTL = tools.Duration{3 * time.Second}

		lock, err := a.Lock("test")
		tools.Ok(t, err)

		_, err = lock.Lock()
		tools.Ok(t, err)

		tools.Assert(t, lock.IsHeld(), "not held")

		time.Sleep(5 * time.Second)

		tools.Assert(t, lock.IsHeld(), "not held")
	})
}
