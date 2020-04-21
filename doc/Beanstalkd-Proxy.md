Beanstalkd Proxy
================

High level flow
---------------

- The cluster-node server exposes the job state machine as a GRPC service, allowing a client to make necessary transitions. 
- Beanstalkd-proxy server, which is a client of the cluster-node makes these transitions on behalf of its clients.
- The GRPC requests should be made to the cluster-node which is the Raft leader. 
- The Raft leader cluster-node internally runs a ticker (executing every second), that updates reservations, timeouts etc. All updates from these are routed to the appropriate beanstalkd-proxy via a GRPC streaming method from the raft leader to the beanstalkd-proxy.

Connectivity
------------

### Problem 

Example: Consider a setup of a three cluster-node setup, and a beanstalkd (proxy) server as a client to this cluster. We have to solve the following problem.
- The beanstalkd-proxy is given addresses to these three cluster-nodes. It must identify the leader, so that all GRPC requests are made to the leader.
- If the leadership is re-assigned, then beanstalkd-proxy must again identify the leader and re-connect to the leader. -
- If there is no leader, beanstalkd-proxy must have a retry policy in place
- During the period of disruption, beanstalkd-proxy's clients see minimal disruption. Especially, with streaming reservations

### Approach using GRPC health check

- The proposed approach is to use [GRPC health check protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md) for the cluster-node's job state machine GRPC service.

- A GRPC service can report one of the health states (UNKNOWN, SERVING, NOT_SERVING)

```
  enum ServingStatus {
    // Indicates that the service's serving status is unknown
    UNKNOWN = 0;

    // Indicates that the service is running and ready to serve requests
    SERVING = 1;

    // Indicates that the service is running and not ready to take traffic
    NOT_SERVING = 2;
  }
```
  Here, only the leader reports a ServingStatus of SERVING, and the other cluster-nodes report NON_SERVING


- A GRPC client, here like the beanstalkd-proxy registers a round-robin loadbalancer with the addresses of all the GRPC cluster-node, with health checking enabled. Here, the health probe should automatically ensure that the request is routed to the leader. 

- Drawback: A leader change can trigger some of the streaming reservations to be missed. One approach is to have the StreamingGRPC setup with all the cluster-node (independent of the health serving status). This minimizes the streaming reservations.