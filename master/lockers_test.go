package master

import (
	"github.com/coldog/sked/tools"

	"testing"
	"github.com/coldog/sked/backends/mock"
)

func TestLockers(t *testing.T) {
	a := mock.NewMockApi()

	lockers := NewSchedulerLocks(a)

	lock1, err := lockers.Lock("testing")
	tools.Ok(t, err)

	tools.Assert(t, lock1 != nil, "lock 1 is nil")
	tools.Assert(t, lock1.IsHeld(), "lock 1 is not held")

	lock2, err := lockers.Lock("testing")

	tools.Assert(t, lock2.IsHeld(), "lock 2 is not held")

	lockers.Unlock("testing")
	tools.Assert(t, !lock2.IsHeld(), "lock 2 is held")
	tools.Assert(t, !lock1.IsHeld(), "lock 2 is held")
}
