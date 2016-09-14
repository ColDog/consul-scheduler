package main

import (
	"time"

	"golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"fmt"
	"github.com/coldog/sked/backends"
	"github.com/coldog/sked/api"
)

func NewEtcdApi(prefix string) *EtcdApi {
	c, err := client.New(client.Config{
		Endpoints:               []string{"http://127.0.0.1:2379"},
		Transport:               client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	})
	if err != nil {
		panic(err)
	}

	return &EtcdApi{
		prefix: prefix,
		kv: client.NewKeysAPI(c),
	}
}

type EtcdApi struct {
	kv client.KeysAPI
	prefix string
}

func (a *EtcdApi) put(key string, val []byte) error {
	_, err := a.kv.Set(context.Background(), a.prefix+key, string(val), nil)
	return err
}

func (a *EtcdApi) del(key string) error {
	_, err := a.kv.Delete(context.Background(), a.prefix+key, nil)
	return err
}

func (a *EtcdApi) get(key string) ([]byte, error) {
	res, err := a.kv.Get(context.Background(), a.prefix+key, &client.GetOptions{Recursive: false})
	if err != nil {
		return nil, err
	}

	if res == nil || res.Node.Value == "" {
		return nil, api.ErrNotFound
	}

	return []byte(res.Node.Value), nil
}

func (a *EtcdApi) list(key string, each func(val []byte) error) error {
	res, err := a.kv.Get(context.Background(), a.prefix+key, &client.GetOptions{Recursive: true})
	if err != nil {
		return err
	}

	for _, v := range res.Node.Nodes {
		err := each([]byte(v.Value))
		if err != nil {
			return err
		}
	}
	return nil
}

// ==> CLUSTER operations

func (a *EtcdApi) ListClusters() (clusters []*api.Cluster, err error) {
	err = a.list("config/clusters", func(v []byte) error {
		c := &api.Cluster{}
		backends.Decode(v, c)
		clusters = append(clusters, c)
		return nil
	})
	return
}

func (a *EtcdApi) GetCluster(id string) (c *api.Cluster, err error) {
	v, err := a.get("config/clusters/" + id)
	if err != nil {
		return nil, err
	}

	backends.Decode(v, c)
	return
}

func (a *EtcdApi) PutCluster(c *api.Cluster) error {
	return a.put("config/clusters/" + c.Name, backends.Encode(c))
}

func (a *EtcdApi) DelCluster(id string) error {
	return a.del("config/clusters/" + id)
}

