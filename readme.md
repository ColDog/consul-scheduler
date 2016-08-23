# Consul Scheduler


## Overview
A scheduler that uses Consul's health checking capabilities as well as it's key value store and locking to build a
distributed and robust container scheduler that schedule's tasks which can utilize the full power of Consul's health
checking and service discovery.

*This project is currently a very early work in progress, please help us make it great!*

### Project Goals

1. Simple to run: One single binary which does not store any state outside of Consul and can run anywhere golang can run.
2. Scale as far as consul: Use Consul's distributed locking and KV store to scale to multiple datacenters.
3. Healing: Integrate deeply with Consul's health checking to restart tasks quickly.
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
        1. [Docker Executor](#docker-executor)
        2. [Bash Executor](#bash-executor)
6. [Deploying](#deploying)
7. [Roadmap](#roadmap)

## Quickstart

1. Download Consul: https://www.consul.io/intro/getting-started/install.html
3. Start Consul: `consul agent -dev -ui -bind=127.0.0.1`
3. Download Consul-Scheduler: https://github.com/ColDog/consul-scheduler/releases and `cd` into the directory
4. Start Consul-Scheduler: `./consul-scheduler combined` (start in combined mode)
5. Load an example: `./consul-scheduler apply -f examples/hello-world.yml`
6. Watch the output and see it schedule the tasks!


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
this includes an overview of the

### Scheduling

Schedulers are set up to provide parallel execution and fault tolerance as well as the ability to write your own scheduler
very easily. The scheduler process first maintains a lock through the storage provider (ie. Consul) on a particular cluster
that it wants to schedule. Cluster's are scheduled independently. The scheduler can then begin creating and removing tasks
inside the storage provider and the agents will pick up these changes and execute start and stop requests. Through this
model the scheduler is highly decoupled from the agents. Any process that can write to the storage provider can become a
scheduler.

Concurrent modifications are handled through using the locking provided by the storage provider. You can run multiple
scheduler processes but only one should be able to handle scheduling for a cluster at a given point in time, this enables
a few of the scheduler processes to fail before the cluster cannot be scheduled.

Writing your own scheduler is easy. Name your scheduler and update the configuration in your clusters to use this name.
Then retrieve a lock on the `schedulers/<cluster_name>` key and begin scheduling if you have the lock. Every object is
json serialized so if your scheduler keeps to the same schema the agent will be able to decode and use your issued tasks.

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

    tasks:          state/tasks/_/<id>
    task by host:   state/tasks/<host>/<id>

Note that tasks are stored both by ID as well as by host. This allows for efficient iteration over the tasks either by
cluster and service or either by host.

## Configuring

This section goes over the json format for storing configuring each object.

_Until a 1.0 release is reached, the json format could change_

Configuration can be added to the cluster by using the CLI provided with the main binary. There is only one main command
`apply` which can take a yml file with the following setup. Follow the json structure for each object below in the YAML
setup.

```yaml
clusters:
    - <cluster-object>

services:
    - <service-object>

task_definitions:
    - <task-definition-object>
```

#### Clusters
```json
{
  "name": "default",          // a unique name for this cluster
  "scheduler": "default",     // the name of the scheduler to use
  "services": ["helloworld"]  // a list of services that this cluster should run
}
```

#### Services
```json
{
  "name": "helloworld",       // a unique name for this service
  "task_name": "helloworld",  // the task name this service should run
  "task_version": 2,          // the task version this service should run
  "desired": 4,               // the desired amount of tasks
  "min": 3,                   // the minimum amount the scheduler can drop to
  "max": 4                    // the maximum amount the scheduler can issue
}
```

#### Task Definitions
```json
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
        "container_port": 80,
        "image": "tutum/hello-world",
        "name": "helloworld"
      }
    }
  ],

  // checks is an array of health checks that should be passed onto consul, these use the same schema
  // as a consul health check, the only addition is the "add_provided_port" field which will tell the
  // scheduler to add the provided port for a task to the end of the tcp or http health check upon
  // scheduling.
  "checks": [
    {
      "name": null,
      "http": "http://127.0.0.1",
      "tcp": null,
      "script": null,
      "add_provided_port": true,
      "interval": "10s",
      "timeout": null,
      "ttl": null
    }
  ]
}
```

##### Docker Executor
```json
{
  "image": "ubuntu",            // docker image
  "cmd": ["consul-scheduler"],  // commands
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
```json
{
  "start": ["echo start"],            // a list of bash commands to start a task
  "stop": ["echo stop"],              // a list of bash commands to stop a task
  "env": ["TEST=true"],               // env to use
  "artifact": "http://download.com",  // download an artifact
  "download_dir": "/usr/local/bin"    // location to download into
}
```

## Deploying
To deploy the scheduler, download a release on each machine

## Roadmap

- [x] examples with basic tests
- [x] basic documentation
- [ ] ci setup
- [ ] full api documentation
- [ ] full test coverage
- [ ] additional scheduler example
- [ ] another backend (ie etcd or zookeeper)
- [ ] large scale testing 1000s of hosts
