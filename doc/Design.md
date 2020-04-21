- [coolbeans](#coolbeans)
    - [Consensus](#consensus)
    - [Beanstalkd Job life-cycle](#beanstalkd-job-lifecycle)
    - [Node Design ](#node-design)

Coolbeans
=========

This document contains design notes for coolbeans. To summarize, Coolbeans is a  distributed work queue that implements the [beanstalkd protocol](https://github.com/beanstalkd/beanstalkd/blob/master/doc/protocol.txt). 

Coolbeans primarily differs from beanstalkd in that it allows the work queue to be replicated across multiple machines. Coolbeans, uses the [Raft consensus algorithm](https://raft.github.io/) to consistently replicate the job state transitions across multiple machines.  

Consensus
---------

Concept: A state machine consists of a collection of states, a collection of transitions between states, and a current state. A transition to a new current state happens in response to an issued operation and produces an output. In a deterministic state machine, for any state and operation, the transition enabled by the operation is unique and the output is a function only of the state and the operation. 

In essence, a replicated state machine is realized as follows: A collection of replicas of a deterministic state machine are created. The replicas are then provided with the same sequence of operations, so they go through the same sequence of state transitions and end up in the same state and produce the same sequence of outputs. Consensus is the process of agreeing on one result among a group of participants. Specifically, these participants agree on the transitions to made on the replicated state machine. Refer [this article](http://www.cs.cornell.edu/courses/cs7412/2011sp/paxos.pdf) for a detailed discussion on the consensus problem.

Raft provides a consensus protocol for state machine replication in a distributed asynchronous environment.

Beanstalkd Job life-cycle
------------------------

The diagram basically shows the life-cycle of a beanstalkd job, the different states & transitions. 

Source: This diagram below has been adapted from the beanstalkd protocol document found [here](https://github.com/beanstalkd/beanstalkd/blob/master/doc/protocol.txt). The document is fairly extensive and explains all the states thoroughly.

```
  put with delay               release with delay
  ----------------> [DELAYED] <------------.
                        |                   |  touch (extend ttr)
                        | timeout/ttr       | .----.
                        |                   | |    |
   put                  v     reserve       | |    v   delete
  -----------------> [READY] ------------> [RESERVED] --------> *poof*
                       ^  ^                | | |
                       |  ^\  release      | | |
                       |   \ `-------------' | |
                       |    \                | |
                       |     \  timeout/ttr  , |
                       |      `--------------  |
                       |                       |
                       | kick                  |
                       |                       |
                       |       bury            |
                    [BURIED] <-----------------'
                       |
                       |  delete
                        `--------> *poof*

```

Node Design
-----------

Terminology:

| Participant               | Description  |
|---------------------------|--------------|
| Client                    | A client of the beanstalkd service, the client interacts with the beanstalkd server over TCP using the beanstalkd protocol.  |
| Beanstalkd (Proxy) Server | Represents the coolbeans beanstalkd server, which interacts with the client via TCP using the beanstalkd protocol. The beanstalkd server is a client to the cluster-node server, and works like a proxy. |
| Cluster-Node Server       | The cluster node server maintains the [job life-cycle]((#beanstalkd-job-lifecycle). The node is essentially a part of a Raft cluster, where job state transitions are replicated consistently using the Raft consensus protocol. The cluster-node server exposes the job state machine as a GRPC service |


~~~
    ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
             Client                
    └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
                │
                v
    ┌─────────────────────────────┐
       Beanstalkd Protocol (TCP)
    └─────────────────────────────┘
    ┌─────────────────────────────┐
       BEANSTALKD (Proxy) SERVER  
    └─────────────────────────────┘
    ┌─────────────────────────────┐
         Proxy Client (GRPC)      
    └─────────────────────────────┘
                |
                v
    ┌─────────────────────────────┐
                 GRPC             
    └─────────────────────────────┘
    ┌─────────────────────────────┐ 
         CLUSTER-NODE SERVER      
    └─────────────────────────────┘
    ┌───────────────────────────────────────────────┐ 
    │          Raft Leader (hashicorp/raft)          --------> Raft Proto.  ---->  Other Raft node(s) 
    └───────────────────────────────────────────────┘ 
    ┌───────────────────────────────────────────────┐
    │              beanstalkd state                 │
    └───────────────────────────────────────────────┘
    ┌───────────────────────────────────────────────┐
    │                 RAM or disk                   │
    └───────────────────────────────────────────────┘
~~~

The above diagram shows the high level node design. <sup>[1](#f1)</sup>

- A setup would have three cluster-nodes resilient to the failure of a single node.   
- One could have a single beanstalkd proxy server per machine/node or pod, similar to [this](https://cloud.google.com/sql/docs/mysql/sql-proxy). Or  can have multiple clients on different machine/node/pods connect to a fleet of beanstalkd-proxy servers

---

<a name="f1">1</a>: Node diagram adapted from node design diagram in [rqlite](https://github.com/rqlite/rqlite/blob/master/DOC/DESIGN.md).

