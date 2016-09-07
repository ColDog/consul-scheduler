# Sked

A Container scheduler and orchestration tool that supports multiple container runtimes. Sked aims to be easy to run, 
simple to operate and to integrate with your favourite tools. Setting up a cluster should be easy.

##### Starting up a cluster
```
$ sked apply -f examples/hello-world-cluster.yml
```

##### Watching your tasks
```
$ sked tasks
id                         host                           rejected     cluster     service        task def       version     healthy     
-----------------------------------------------------------------------------------------------------------------------------------------
default-helloworld-4-0     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-1     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-2     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-3     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-4     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-5     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false       
default-helloworld-4-6     Colins-MacBook-Pro-2.local     false        default     helloworld     helloworld     4           false
```

[![CircleCI](https://circleci.com/gh/ColDog/sked.svg?style=svg)](https://circleci.com/gh/ColDog/sked)

## Overview
A scheduler that uses Consul's health checking capabilities as well as it's key value store and locking to build a
distributed and robust container scheduler that schedule's tasks which can utilize the full power of Consul's health
checking and service discovery.

*This project is currently a very early work in progress, please help us make it great!*

### Project Goals

1. Simple to run: One single binary which does not store any state outside of the backend.
2. Scaling: The scheduler should be able to parallelize it's scheduling capabilities.
3. Healing: Health checks are a first class citizen, they should be integrated deeply with the scheduler.
4. Extensible: Write your own scheduler and deploy it easily in any language.


## Contents

1. [Quickstart](#quickstart)
2. [Definitions](#definitions)
    1. [Objects](#objects)
    2. [Concepts](#concepts)
    3. [Processes](#processes)
3. [Architecture](#architecture)
    1. [Agents](#agents)
    2. [Scheduler](#scheduler)
4. [Storage](#storage)
    1. [Desired State](#desired-state)
    2. [Issued State](#issued-state)
5. [Configuring](#configuring)
    1. [Services](#services)
    2. [Clusters](#clustes)
    3. [Task Definitions](#task-definitions)
    3. [Tasks](#tasks)
    3. [Executors](#tasks)
        1. [Docker Executor](#docker-executor)
        2. [Bash Executor](#bash-executor)
7. [Roadmap](#roadmap)

## Quickstart

1. Download Consul: https://www.consul.io/intro/getting-started/install.html
3. Start Consul: `consul agent -dev -ui -bind=127.0.0.1` (development mode means we don't need a cluster)
3. Download Sked: https://github.com/ColDog/sked/releases
4. Run the binary: `./sked combined` (start in combined mode)
6. Copy an example from the examples directory to your local machine
7. Run `./sked apply -f <example_file>.yml`
8. Watch it schedule the tasks!

## Rationale

Why build another scheduler? Currently there are quite a few projects that come to mind as being production ready solutions
for scheduling containers. Kubernetes is probably the oldest and most well known out of these projects, Amazon's offering
ECS is also a fully managed solution that is being widely used while Hashicorp has released a scheduler called Nomad which
can scale to over 1 million containers in their tests.

Sked is designed with simplicity in mind. It is an exercise in making the simplest scheduler possible while involving the
least amount of setup and importantly a very basic operational understanding. Sked is completely stateless and refers to
consul or etcd (forthcoming) to store all state. The bare state is exposed to the operator allowing the operator to make
on the fly changes and get total introspection into the cluster state with their own tools. Sked is also distributed, it
uses the locking inherit in the chosen backend to avoid scheduling conflicts but ultimately uses a version of optimistic
concurrency to allow for fast, fault tolerant and distributed scheduling.


## Definitions

### Services

Health Check Provider: A service that maintains health checks on the cluster and can report the health of a given _task_.

Storage Provider: Something that provides a strongly consistent storage solution.

### Objects

Container: An executable image, either docker or other, that can be run.

Host: A physical machine where _containers_ can be run.

Task Definition: A collection of _containers_ that can be health checked and should run together.

Service: An object containing the configuration for running a given _task definition_.

Cluster: A logical grouping of _services_.

Task: A running instance of a _task definition_, associated with a _service_, _cluster_ and most importantly a _host_.

Executor: The main configuration for a container that tells the agent how to start and stop it.

### Concepts

Desired State: The desired state of all clusters described by the end user.

Actual State: The state of the cluster as described by the health checker (Consul), which includes the desired state plus information
about whether a task is running or not.

Issued State: The desired state plus which hosts everything should be running on.

### Processes

Agent: A process that monitors the _issued state_ and attempts to match it to the _actual state_ for a given host.

Scheduler: A process that monitors the _desired state_ for a given cluster and creates the _issued state_.

## Architecture

### Agents

Agents monitor constantly the issued state by the scheduler and the actual state from the health checker. If anything
changes then the agent will start or stop tasks depending on the difference between the two states. The tasks are started
and stopped depending on the executor provided in the configuration. Currently agents only support the bash and docker
executors although more are on the roadmap.

Agents are very memory efficient and maintain a very small amount of local state only which does not need to be persisted.
As a result they are very simple to run in production. They also offer parallel execution starting of tasks so they are
fast and responsive to changes in configuration and health.

The agent will also broadcast it's state to the storage provider to be used by the scheduler in making scheduling decisions.
this includes an overview of the memory available, ports currently allocated and disk space used by the host machine. Agents
also have the power to reject a task scheduled by the scheduler. If, for example, a port conflict was accidentally created,
an agent will reject the task, triggering the scheduler which will have a second shot at scheduling the conflicting task.

Agents can run without the scheduling backend and do not require any of the desired state to function. They merely require
the issued state to be present in the storage backend to function. This allows for pluggable scheduling, or even manual
scheduling of tasks. Simply follow the json structure and the key hierarchy outlined in the docs and the agents will be
able to run the provided tasks.

### Scheduling

When the `scheduler` command is passed to the binary this will start a process that monitors the configuration and the hosts
in the defined cluster and dispatches requests to schedule when a change in the configuration is noticed. Schedulers run
on a per cluster and service basis. The master process monitors all resources in the defined cluster and issues requests
to schedule for a given service if needed.

When a scheduler begins to schedule for a given service, it first attempts to lock that service and cluster. If a lock
cannot be retrieved immediately, the scheduler will exit and wait for another request. This allows for us to run multiple
schedulers throughout the cluster for fault tolerance and parallel scheduling.

Since schedulers work parallel to one another, they may schedule a task on a host where the port was already reserved by
another task, or they may miscalculate the resources of a host since other scheduler processes have already allocated that
memory or cpu. Agents are able to reject a task, which triggers another scheduling and forces the scheduler to rebalance
the tasks across the hosts accordingly.

#### Writing Your Own Scheduler

Your own scheduling system can be implemented using the same format. A process can run and monitor the cluster state while
creating and removing tasks as needed. Fundamentally, the agent's and the schedulers are separate processes and can run
entirely independently. All the agent cares about is that the json posted to the backend is readable and conforms to the
same key mapping.

Overall, all your scheduler needs to do is create tasks readable by the agent under the following key:

    /state/hosts/<host_name>/<task_id> => {task}

All of the locking and monitoring of the cluster is ancillary and necessary to get distributed fault tolerant scheduling,
however if you want to quickly hand roll a solution or have specific scheduling requirements this could be a viable
alternative.

## Storage

The key schema used in the storage provider is defined below. The keyspace is separated into two basic quadrants. The
`config/*` space represents the _desired state_ for all clusters. Any changes on this keyspace mean a change in the user
defined configuration. The _issued state_ for all clusters is stored under the `state/*` keyspace. Any change in this
keyspace means that the scheduler has issued a change to a task that should be started or stopped by the agents.

#### Desired State

The storage format for the desired state is as follows:

    clusters:           config/clusters/<name>
    services:           config/services/<name>
    task definitions:   config/task_definitions/<name>/<version>
    host:               config/hosts/<name>

#### Issued State

Task ID's are comprised of the following schema:

    <cluster_name>-<service_name>-<version>-<instance>

Tasks, making up the issued state are stored as follows:

    tasks:          state/tasks/<task_id>
    tasks:          state/hosts/<host_id>/<task_id>

The tasks by host allow for efficient queries from the agent's perspective to get a quick picture of all the task ID's
that should be under it's control.

## Configuring

This section goes over the json format for storing configuring each object.

_Until a 1.0 release is reached, the json format could change_

Configuration can be added to the cluster by using the CLI provided with the main binary. There is only one main command
`apply` which can take a yml file with the following setup. Follow the json structure for each object below in the YAML
setup. You can also check out the examples directory for some examples for bringing up a cluster.

You can insert directly your objects into the storage backend, however you will miss out on some key validations that may
break your applications. For example, the 'grace_period' field is set to a sensible default by the validations but if left
blank will only give your application a 0s startup period.

```yaml
clusters:
    - <cluster-object>

services:
    - <service-object>

task_definitions:
    - <task-definition-object>
```

#### Clusters
```javascript
{
  "name": "default",          // a unique name for this cluster
  "services": ["helloworld"]  // a list of services that this cluster should run
}
```

#### Services
```javascript
{
  "name": "helloworld",       // a unique name for this service
  "task_name": "helloworld",  // the task name this service should run
  "scheduler": "default",     // the scheduler that should be used
  "task_version": 2,          // the task version this service should run
  "desired": 4,               // the desired amount of tasks
  "min": 3,                   // the minimum amount the scheduler can drop to
  "max": 4                    // the maximum amount the scheduler can issue
}
```

#### Task Definitions
```javascript
{
  "name": "helloworld",                   // a unique name for the task definition
  "version": 2,                           // the task definition version, for seamless upgrades
  "provide_port": true,                   // tells the scheduler to provide a port
  "port": 0,                              // fix a port for this task
  "tags": ["urlprefix-/helloworld"],      // a list of tags passed on to consul
  "memory": 0,                            // the amount of memory used by this task
  "cpu_units": 0,                         // the number of cpu units used by this task

  // the containers array is a list of containers that will be executed by the agent when starting
  // and stopping the task
  "containers": [
    {
      "name": "helloworld",               // a unique name for the container
      "type": "docker",                   // the type of executor currently: docker, bash
      "setup": ["echo setup"],            // a list of bash commands to setup a container
      "executor": {                       // individual configuration for the executore
              // see executor object
      },

      // checks is an array of health checks that should be passed onto consul, these use the same schema
      // as a consul health check, the only addition is the $PROVIDED_PORT variable which will tell the
      // scheduler to add the provided port for a task to the end of the tcp or http health check upon
      // scheduling.
      "checks": [
          {
            "name": null,
            "http": "http://127.0.0.1:$PROVIDED_PORT",
            "tcp": null,
            "script": null,
            "interval": "10s",
            "timeout": null,
            "ttl": null
          }
        ]
    }
  ]
}
```

#### Tasks

Tasks are serialized with the full task definition.

```javascript
{
  "cluster": {
    "name": "default",
    "datacenter": "",
    "services": [
      "helloworld"
    ],
    "hosts": null
  },
  "task_definition": {
    "name": "helloworld",
    "version": 2,
    "provide_port": true,
    "port": 0,
    "tags": [
      "urlprefix-\/helloworld"
    ],
    "containers": [
      {
        "name": "helloworld",
        "type": "docker",
        "executor": {
          "container_port": 80,
          "image": "tutum\/hello-world",
          "name": "helloworld"
        },
        "setup": null,
        "checks": [
          {
            "id": "",
            "name": "",
            "http": "http:\/\/127.0.0.1:$PROVIDED_PORT",
            "tcp": "",
            "script": "",
            "interval": "10s",
            "timeout": "",
            "ttl": ""
          }
        ],
        "memory": 0,
        "cpu_units": 0,
        "disk_use": 0
      }
    ],
    "grace_period": 60000000000,
    "max_attempts": 10
  },
  "service": "helloworld",
  "instance": 0,
  "port": 20000,
  "host": "Colins-MacBook-Pro-2.local",
  "scheduled": true,
  "rejected": false,
  "reject_reason": ""
}
```

#### Executors

Executors are handled dynamically internally as opaque blobs that are parsed by a handler given the executor 'type' in the
task definition. The interface that an executor must implement includes only three methods:

```go
type Executor interface {
	StartTask(t *Task) error
	StopTask(t *Task) error
	ReservedPorts() []uint
}
```

These methods allow the agent to start the task and stop the task. If the `StartTask` method errors the agent will not
register the service for health checks or in the service discovery backend. The reserved ports is a list of ports used
by the scheduler to determine if this executor conflicts with a host.

##### Docker Executor
```javascript
{
  "image": "ubuntu",            // docker image
  "cmd": ["sked"],  // commands
  "entry": "",                  // docker entrypoint

  // when a provided port is mapped on the task definition and the container port is present
  // the task will map the provided port to the container port.
  "container_port": 3000,
  "ports": ["5000:5000"],       // additional ports to map
  "env": [],                    // docker environment
  "work_dir": "/",              // docker work directory
  "volumes": [],                // volumes to attach
  "flags": ["--net=host"]       // additional flags passed to docker run
}
```

##### Bash Executor
```javascript
{
  "start": "echo start",              // a shell commands to start a task
  "stop": "echo stop",                // a shell commands to stop a task
  "env": ["TEST=true"],               // env to use
  "artifact": "http://download.com",  // download an artifact
  "download_dir": "/usr/local/bin"    // location to download into
}
```

## Roadmap

- [x] examples with basic tests
- [x] basic documentation
- [x] health checking
- [ ] etcd backend
- [ ] full test coverage
- [ ] full api documentation
- [ ] dynamodb backend
- [ ] scheduling constraints
