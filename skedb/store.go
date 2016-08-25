package skedb

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"
	"github.com/hashicorp/go-memdb"

	"fmt"
	"time"
	"net/http"
	"encoding/json"
)

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"task": &memdb.TableSchema{
			Name: "task",
			Indexes: map[string]*memdb.IndexSchema{
				"id": &memdb.IndexSchema{
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "ID"},
				},
				"name": &memdb.IndexSchema{
					Name:    "name",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Name"},
				},
				"host": &memdb.IndexSchema{
					Name:    "host",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Host"},
				},
				"cluster": &memdb.IndexSchema{
					Name:    "cluster",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Cluster"},
				},
				"service": &memdb.IndexSchema{
					Name:    "service",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Service"},
				},
				"task_definition": &memdb.IndexSchema{
					Name:    "task_definition",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "TaskDefinition"},
				},
				"version": &memdb.IndexSchema{
					Name:    "version",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Version"},
				},
				"task_and_version": &memdb.IndexSchema{
					Name:    "task_and_version",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "TaskAndVersion"},
				},
				"service_and_version": &memdb.IndexSchema{
					Name:    "service_and_version",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "ServiceAndVersion"},
				},
				"scheduled": &memdb.IndexSchema{
					Name:    "scheduled",
					Unique:  false,
					Indexer: &memdb.FieldSetIndex{Field: "Scheduled"},
				},
			},
		},
	},
}

type Task struct {
	ID                string
	Name              string
	Cluster           string
	Service           string
	Host              string
	TaskDefinition    string
	Version           string
	TaskAndVersion    string
	ServiceAndVersion string
	Scheduled         bool
	api               api.SchedulerApi
}

func (t Task) Task() (*api.Task, error) {
	return t.api.GetTask(t.ID)
}

type Iterator func(t Task) error

func NewSkedDB(a api.SchedulerApi) *SkedDB {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}

	return &SkedDB{api: a, db: db}
}

// This keeps an up to date picture of all the scheduled tasks
type SkedDB struct {
	api api.SchedulerApi
	db  *memdb.MemDB
}

func (db *SkedDB) ForEach(index string, value interface{}, iter Iterator) error {
	tx := db.db.Txn(false)
	defer tx.Abort()

	rs, err := tx.Get("task", index, value)
	if err != nil {
		return err
	}

	for raw := rs.Next(); raw != nil; raw = rs.Next() {
		t := raw.(Task)
		err := iter(t)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *SkedDB) Count(index string, value interface{}) (int, error) {
	count := 0
	err := db.ForEach(index, value, func(t Task) error {
		count++
		return nil
	})

	return count, err
}

func (db *SkedDB) All(index string, value interface{}) ([]Task, error) {
	tasks := []Task{}
	err := db.ForEach(index, value, func(t Task) error {
		tasks = append(tasks, t)
		return nil
	})

	return tasks, err
}

func (db *SkedDB) Put(t *api.Task) error {
	txn := db.db.Txn(true)

	ti := Task{
		ID:                t.Id(),
		Name:              t.Name(),
		Cluster:           t.Cluster.Name,
		Service:           t.Service,
		Host:              t.Host,
		Scheduled:         t.Scheduled,
		Version:           fmt.Sprintf("%d", t.TaskDefinition.Version),
		TaskAndVersion:    fmt.Sprintf("%s-%d", t.TaskDefinition.Name, t.TaskDefinition.Version),
		ServiceAndVersion: fmt.Sprintf("%s-%d", t.Service, t.TaskDefinition.Version),
		TaskDefinition:    t.TaskDefinition.Name,
		api:               db.api,
	}

	if err := txn.Insert("task", ti); err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func (db *SkedDB) RegisterRoutes() {
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var res interface{}

		for _, idx := range schema.Tables["task"].Indexes {
			val := r.URL.Query().Get(idx.Name)
			if val != "" {
				tasks, err := db.All(idx.Name, val)
				if err != nil {
					res = err
				} else {
					res = tasks
				}

				break
			}
		}

		data, err := json.Marshal(res)
		if err != nil {
			panic(err)
		}
		w.Write(data)
	})
}

func (db *SkedDB) Del(taskId string) error {
	tx := db.db.Txn(true)
	err := tx.Delete("task", taskId)
	if err != nil {
		tx.Abort()
		return err
	}
	tx.Commit()
	return nil
}

func (db *SkedDB) Sync() error {
	t1 := time.Now().UnixNano()

	tasks, err := db.api.ListTasks(&api.TaskQueryOpts{})
	if err != nil {
		return err
	}

	for _, t := range tasks {
		db.Put(t)
	}

	t2 := time.Now().UnixNano()
	log.WithField("time", t2-t1).WithField("seconds", float64(t2-t1)/1000000000.00).Info("[skedb] synced")
	return nil
}

func (db *SkedDB) Listener() {
	listen := make(chan string, 10)
	db.api.Subscribe("skedb", "config::*", listen)
	defer db.api.UnSubscribe("skedb")

	for range listen {
		db.Sync()
	}
}
