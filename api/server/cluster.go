package server

import (
	"github.com/coldog/sked/server"
	"github.com/coldog/sked/api"
)

type ClusterResource struct {
	api api.SchedulerApi
}

func (r *ClusterResource) Index(c *server.Context) (int, interface{}) {
	data, err := r.api.ListClusters()
	if err != nil {
		return 500, err
	}

	return 200, data
}

func (r *ClusterResource) Show(c *server.Context) (int, interface{}) {
	data, err := r.api.GetCluster(c.Get("id"))
	if err != nil {
		return 404, err
	}

	return 200, data
}

func (r *ClusterResource) Create(c *server.Context) (int, interface{}) {
	cluster := &api.Cluster{}
	err := c.Unmarshal(cluster)
	if err != nil {
		return 400, err
	}

	errs := cluster.Validate(r.api)
	if len(errs) > 0 {
		return 400, errs
	}

	err = r.api.PutCluster(cluster)
	if err != nil {
		return 400, err
	}

	return 200, cluster
}

func (r *ClusterResource) Update(c *server.Context) (int, interface{}) {
	cluster, err := r.api.GetCluster(c.Get("id"))
	if err != nil {
		return 404, err
	}

	err = c.Unmarshal(cluster)
	if err != nil {
		return 400, err
	}

	errs := cluster.Validate(r.api)
	if len(errs) > 0 {
		return 400, errs
	}

	err = r.api.PutCluster(cluster)
	if err != nil {
		return 400, err
	}

	return 200, cluster
}

func (r *ClusterResource) Destroy(c *server.Context) (int, interface{}) {
	err := r.api.DelCluster(c.Get("id"))
	if err != nil {
		return 400, err
	}

	return 200, c.Get("id")
}
