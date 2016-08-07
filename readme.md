# Architecture

### Definitions

ClusterConfig:
The user defined configuration

DesiredState:
The schedulers definition of what the cluster should look like

ActualState:
Consul's definition of what the cluster looks like

Scheduler:
A process that creates the desired state.

Cluster:
A collection of services.

Service:
A configuration for running task definitions.

Task Definition:
A definition of a task with a port, priority, multiple containers and health checks.

Container:
An executable process that can be invoked.

Task:
A task definition tied to a host, cluster and service.


### Overview

Task definitions are a collection of containers and health checks. Task definitions are run by services in clusters. A
Task is an instance of a running task definition with an associated cluster and service. The task is given a
unique id and also a service name. The task is then registered in consul as a service along with it's health checks, host,
and configured or assigned port. The full power of consul's service discovery can then be utilized.

Users create task definitions, services and clusters and register them with the server.

The scheduler takes the cluster configuration and the current state and creates a desired state. This state is saved in
consul with a specific format readable to the agent. Schedulers can be implemented very dynamically in many languages as
they only have to work over http.

The desired state is read by each agent who attempt to bring their localhost up to par with the desired state for their
given host. Each agent monitors the desired state and compares it against the actual state for their localhost and takes
action to start or stop containers to bring them in line with the desired state.

Agents may trigger the scheduler based on multiple failures of a given task or if their performance is worse than expected.
When executing start or stop commands for tasks, agents queue these tasks if the conditions specified in the related
service are not met. This provides for lightweight scheduling, while the agents help enforce a minimum number of running
tasks.


### Scenarios

Bad Build:

1) user registers a new task version with a new image
2) scheduler places it on a host
3) host picks up this key and attempts to schedule this task
4) another host notices that the old build is stale and should be removed
5) the health check begins to fail on the new build
6) if the minimum amount of tasks in the service are not healthy, the other host will not remove the service

Host Failure:

1) the scheduler is triggered and rebalances the tasks across the newly available hosts
2) each agent picks up the new tasks and starts them concurrently

High Memory:

1) an agent notices that the host it's running on is out of memory
2) the agent picks a task and updates it's configuration to run on a different host
3) the new host picks this up and schedules the task


### Consul kv

    cluster/<cluster_name>      => cluster configuration
    service/<service_name>      => service configuration
    task/<task_name>            => task configuration
    host/<host_name>            => host configuration

    state/<host_id>/<task_id>   => instance of a scheduled task for a given host
    state/<task_id>             => set to true if task is currently scheduled
