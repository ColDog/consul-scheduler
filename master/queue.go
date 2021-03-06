package master

func NewSchedulerQueue() *SchedulerQueue {
	q := &SchedulerQueue{
		queue:   make([]interface{}, 0, 200),
		quit:    make(chan struct{}),
		enqueue: make(chan interface{}, 50),
		dequeue: make(chan struct{ res chan interface{} }, 50),
	}

	go q.listen()
	return q
}

// The scheduler queue is a unique queue of strings where updates with the same value increase
// the priority of the value in the queue.
type SchedulerQueue struct {
	queue   []interface{}
	quit    chan struct{}
	enqueue chan interface{}
	dequeue chan struct {
		res chan interface{}
	}
}

func (s *SchedulerQueue) Pop(l chan interface{}) {
	s.dequeue <- struct{ res chan interface{} }{l}
}

func (s *SchedulerQueue) Push(val interface{}) {
	s.enqueue <- val
}

// the queue is implemented as a slice of strings. Upon enqueuing a new string,
// if the value already exists in the queue it is removed, then the value is
// added to the end of the queue.
func (s *SchedulerQueue) doEnqueue(val interface{}) {
	for i, e := range s.queue {
		// remove the latest element from the slice if it exists
		if e == val {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
		}
	}
	s.queue = append(s.queue, val)
}

func (s *SchedulerQueue) listen() {
	for {
		if len(s.queue) > 0 {
			// if the queue has items in it, also listen to the dequeue channel
			select {
			case val := <-s.enqueue:
				s.doEnqueue(val)
			case future := <-s.dequeue:
				future.res <- s.queue[len(s.queue)-1]
				s.queue = s.queue[0 : len(s.queue)-1]

			case <-s.quit:
				return
			}
		} else {
			// if the queue has nothing in it, don't listen in the dequeue channel
			select {
			case val := <-s.enqueue:
				s.doEnqueue(val)
			case <-s.quit:
				return
			}
		}
	}
}

func (s *SchedulerQueue) Stop() {
	close(s.quit)
}
