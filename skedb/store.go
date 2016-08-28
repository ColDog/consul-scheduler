package skedb

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"
	"github.com/hashicorp/go-memdb"

	"fmt"
	"net/http"
	"encoding/json"
	"strings"
)

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"tasks": &memdb.TableSchema{
			Name: "tasks",
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
				"scheduled": &memdb.IndexSchema{
					Name:    "scheduled",
					Unique:  false,
					Indexer: &memdb.FieldSetIndex{Field: "Scheduled"},
				},
			},
		},

		"hosts": &memdb.TableSchema{
			Name: "hosts",
			Indexes: map[string]*memdb.IndexSchema{
				"id": &memdb.IndexSchema{
					Name: "id",
					Unique: true,
					Indexer: &memdb.StringFieldIndex{Field: "ID"},
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

type Host struct {
	ID          string
	Memory        uint64
	DiskSpace     uint64
	CpuUnits      uint64
	MemUsePercent float64
	ReservedPorts []uint
	PortSelection []uint
}

func (t Task) Task() (*api.Task, error) {
	return t.api.GetTask(t.ID)
}

type Iterator func(raw interface{}) error

func NewSkedDB(a api.SchedulerApi) *SkedDB {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}

	s := &SkedDB{api: a, db: db}
	go s.listener()
	return s
}

// This keeps an up to date picture of all the scheduled tasks
type SkedDB struct {
	api api.SchedulerApi
	db  *memdb.MemDB
}

func (db *SkedDB) ForEach(table, index string, value interface{}, iter Iterator) error {
	tx := db.db.Txn(false)
	defer tx.Abort()

	rs, err := tx.Get(table, index, value)
	if err != nil {
		return err
	}

	for raw := rs.Next(); raw != nil; raw = rs.Next() {
		err := iter(raw)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *SkedDB) Count(table, index string, value interface{}) (int, error) {
	count := 0
	err := db.ForEach(table, index, value, func(raw interface{}) error {
		count++
		return nil
	})

	return count, err
}

func (db *SkedDB) All(table, index string, value interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)
	err := db.ForEach(table, index, value, func(raw interface{}) error {
		results = append(results, results)
		return nil
	})
	return results, err
}

func (db *SkedDB) PutHost(h *api.Host) error {
	txn := db.db.Txn(true)

	hi := Host{
		ID: h.Name,
		Memory: h.Memory,
		CpuUnits: h.CpuUnits,
		DiskSpace: h.DiskSpace,
		MemUsePercent: h.MemUsePercent,
		ReservedPorts: h.ReservedPorts,
		PortSelection: h.PortSelection,
	}

	if err := txn.Insert("hosts", hi); err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func (db *SkedDB) PutTask(t *api.Task) error {
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

	if err := txn.Insert("tasks", ti); err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func (db *SkedDB) Del(table, id string) error {
	tx := db.db.Txn(true)
	err := tx.Delete(table, id)
	if err != nil {
		tx.Abort()
		return err
	}
	tx.Commit()
	return nil
}

func (db *SkedDB) RegisterRoutes() {
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var res interface{}

		for _, idx := range schema.Tables["task"].Indexes {
			val := r.URL.Query().Get(idx.Name)
			if val != "" {
				results, err := db.All("tasks", idx.Name, val)
				if err != nil {
					res = err
				} else {
					res = results
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

func (db *SkedDB) listener() {
	listen := make(chan string, 100)
	db.api.Subscribe("skedb", "kv::*", listen)
	defer db.api.UnSubscribe("skedb")

	log.Debug("[skedb] listening")
	for key := range listen {
		key = strings.Replace(key, "kv::", "", 1)
		spl := strings.Split(key, "/")
		fmt.Println("received", key, spl[1], spl[2])
		if spl[1] == "hosts" {
			log.WithField("root", spl[1]).WithField("key", spl[2]).Debug("[skeddb] loading")

		} else if spl[1] == "tasks" {
			log.WithField("root", spl[1]).WithField("key", spl[2]).Debug("[skeddb] loading")
		}
	}
}
