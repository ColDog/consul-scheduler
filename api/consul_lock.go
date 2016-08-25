package api

import (
	"github.com/hashicorp/consul/api"
	"fmt"
)

type ConsulLockWrapper struct {
	lock *api.Lock
	held bool
}

func (l *ConsulLockWrapper) Lock() (<-chan struct{}, error) {
	lc, err := l.lock.Lock(nil)
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
