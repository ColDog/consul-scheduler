package consul

import (
	"github.com/hashicorp/consul/api"
)

type ConsulLockWrapper struct {
	lock   *api.Lock
	held   bool
	quitCh <-chan struct{}
}

func (l *ConsulLockWrapper) Lock() (<-chan struct{}, error) {
	lc, err := l.lock.Lock(nil)
	l.quitCh = lc

	if err != nil {
		return lc, err
	}

	if lc != nil {
		l.held = true
	}

	go func() {
		<-lc
		l.held = false
	}()

	return lc, err
}

func (l *ConsulLockWrapper) IsHeld() bool {
	return l.held
}

func (l *ConsulLockWrapper) Unlock() error {
	l.held = false
	return l.lock.Unlock()
}

func (l *ConsulLockWrapper) QuitChan() <-chan struct{} {
	return l.quitCh
}
