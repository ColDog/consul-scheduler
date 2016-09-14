package etcd

import (
	"github.com/coldog/sked/api"
	"sync"
	"golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"time"
	"fmt"
	"math/rand"
	"github.com/coldog/sked/tools"
)

func (a *EtcdApi) Lock(key string) (api.Lockable, error) {
	return &EtcdLock{
		sess: tools.RandString(120),
		key: a.prefix+key,
		mtx: &sync.Mutex{},
		api: a,
	}, nil
}

// Etcd lock does the following:
// - put a key, if it exists and is not equal to this hosts hostname lock is not held and return
// - if it does not exist put a value and set a TTL
// - refresh the key periodically, lock can only be refreshed if the key is equal to the hostname
type EtcdLock struct {
	key    string
	sess   string
	mtx    *sync.Mutex
	held   bool
	closed chan struct{}
	api    *EtcdApi
}

func (l *EtcdLock) refresh() {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	_, err := l.api.kv.Set(context.Background(), l.key, l.sess, &client.SetOptions{
		TTL: 30 * time.Second,
		Refresh: true,
		PrevValue: l.sess,
	})

	if err != nil {
		l.held = false
		close(l.closed)
	}
}

func (l *EtcdLock) Lock() (<-chan struct{}, error) {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	if l.held {
		return l.closed, nil
	}

	_, err := l.api.kv.Set(context.Background(), l.key, l.sess, &client.SetOptions{
		PrevExist: client.PrevNoExist,
	})
	if err != nil {
		return nil, err
	}

	// we retrieved the lock!
	l.closed = make(chan struct{})
	l.held = true

	go func() {
		for {
			time.Sleep(time.Duration(15 + rand.Intn(10)) * time.Second)
			l.refresh()
		}
	}()

	return l.closed, nil
}

func (l *EtcdLock) IsHeld() bool {
	return l.held
}

func (l *EtcdLock) Unlock() error {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	if !l.held {
		return fmt.Errorf("lock not held")
	}

	_, err := l.api.kv.Delete(context.Background(), l.key, &client.DeleteOptions{
		PrevValue: l.sess,
	})

	l.held = false
	return err
}
