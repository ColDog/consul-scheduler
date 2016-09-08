package mock

import "github.com/coldog/sked/api"

func NewMockLock(k string, a *MockApi) api.Lockable {
	return &MockLock{
		a:   a,
		key: k,
	}
}

type MockLock struct {
	a      *MockApi
	key    string
	held   bool
	quitCh <-chan struct{}
}

func (l *MockLock) Lock() (<-chan struct{}, error) {
	s := make(<-chan struct{})
	l.quitCh = s
	return s, nil
}

func (l *MockLock) IsHeld() bool {
	l.a.lock.Lock()
	defer l.a.lock.Unlock()

	return l.a.locks[l.key]
}

func (l *MockLock) Unlock() error {
	l.a.lock.Lock()
	defer l.a.lock.Unlock()

	l.held = false
	if ok := l.a.locks[l.key]; ok {
		delete(l.a.locks, l.key)
	}
	return nil
}

func (l *MockLock) QuitChan() <-chan struct{} {
	return l.quitCh
}
