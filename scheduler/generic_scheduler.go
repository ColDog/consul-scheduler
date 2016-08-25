package scheduler

import (
	"github.com/coldog/sked/api"
	"time"
)

type Scheduler struct {

}

// The scheduler manages scheduling on a per service basis, dispatching requests for scheduling when needed.
type Master struct {
	Schedulers map[string]
	api    api.SchedulerApi
	quit   chan struct{}
}

func (s *Master) Dispatcher() {
	listener := make(chan string, 100)
	s.api.Subscribe("dispatch", "config::*", listener)
	defer s.api.UnSubscribe("dispatch")

	for {
		select {
		case evt := <-listener:

		case <-s.quit:

		case <-time.After(60 * time.Second):

		}
	}
}

func (s *Master) schedule() {

}