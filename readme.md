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
3. [Scheduling](#scheduling)
4. [Configuring](#configuring)
    a. [Docker Executor](#docker-executor)
    b. [Bash Executor](#bash-executor)
5. [Agents](#agents)
6. [Deploying](#deploying)

### Quickstart

1. Download Consul: https://www.consul.io/intro/getting-started/install.html
3. Start Consul: `consul agent -dev -ui -bind=127.0.0.1`
3. Download Consul-Scheduler: https://github.com/ColDog/consul-scheduler/releases and `cd` into the directory
4. Start Consul-Scheduler: `./consul-scheduler combined` (start in combined mode)
5. Load an example: `./consul-scheduler apply -f examples/hello-world.yml`


### Definitions

#### Objects

Container: An executable image, either docker or other, that can be run.

Host: A physical machine where _containers_ can be run.

Task Definition: A collection of _containers_ that can be health checked and should run together.

Service: An object containing the configuration for running a given _task definition_.

Cluster: A logical grouping of _services_.

Task: A running instance of a _task definition_, associated with a _service_, _cluster_ and most importantly a _host_.

#### Concepts

Desired State: The desired state of all clusters described by the end user.

Actual State: The state of the cluster as described by the health checker (Consul), which includes the desired state plus information
about whether a task is running or not.

Issued State: The desired state plus which hosts everything should be running on.

#### Processes

Agent: A process that monitors the _issued state_ and attempts to match it to the _actual state_ for a given host.

Scheduler: A process that monitors the _desired state_ for a given cluster and creates the _issued state_.


### Scheduling

Schedulers are configured on a

### Configuring

#### YAML File

#### Consul KV

#### Executors

The following is the basic structure of the storage in consul.

    config/cluster/<cluster_name>                   => cluster configuration
    config/service/<service_name>                   => service configuration
    config/task_definitions/<task_name>/<version>   => task configuration
    config/host/<host_name>                         => host configuration

    state/<host_id>/<task_id>                       => task object, sortable by host
    state/_/<task_id>                               => task object, sortable by service

### Agents
