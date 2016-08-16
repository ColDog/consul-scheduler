package tools

import "sync"


func NewEvents() *Events {
	return &Events{
		subscribers: make(map[string] chan struct{}),
		lock: &sync.RWMutex{},
	}
}

type Events struct {
	subscribers map[string] chan struct{}
	lock *sync.RWMutex
}

func (events *Events) Subscribe(key string, ch chan struct{}) {
	events.lock.Lock()
	defer events.lock.Unlock()
	events.subscribers[key] = ch
}

func (events *Events) UnSubscribe(key string) {
	events.lock.Lock()
	defer events.lock.Unlock()
	if _, ok := events.subscribers[key]; ok {
		delete(events.subscribers, key)
	}
}

func (events *Events) Publish() {
	events.lock.RLock()
	defer events.lock.RUnlock()
	for _, ch := range events.subscribers {
		select {
		case ch <- struct {}{}:
		default:
		}
	}
}
