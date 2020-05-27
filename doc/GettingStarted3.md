Getting started - Three node cluster
====================================

In the [Getting started page](./GettingStarted.md), we setup a single node cluster. Here, we setup a three node cluster, which is tolerant to a single node failure. 

Setup cluster
-------------

In this example, we setup a 3 node cluster, which has three cluster node processes and one beanstalkd proxy process.


#### Setup cluster-node bean0

    ./coolbeans  cluster-node --node-id bean0 --root-dir /tmp/multi-bean/bean0 --bootstrap-node-id bean0 --node-peer-addrs 127.0.0.1:11000,127.0.0.1:12000,127.0.0.1:13000 --raft-listen-addr :21000 --node-listen-addr :11000

The above example starts a new cluster node:

- with a unique node-id bean0. 
- the cluster-node starts two listeners, the raft service listens on `:21000` and the GRPC service listens on `:11000`. 
- all data logs and snapshots will be persisted under the directory: `/tmp/multi-bean/bean0`. 
- the bootstrap node, which the default assigned leader during the first time cluster formation, is `bean0`.


#### Setup cluster-nodes bean1 & bean2

    ./coolbeans --quiet cluster-node --node-id bean1 --root-dir /tmp/multi-bean/bean1 --bootstrap-node-id bean0 --node-peer-addrs 127.0.0.1:11000,127.0.0.1:12000,127.0.0.1:13000 --raft-listen-addr :22000 --node-listen-addr :12000

    ./coolbeans  cluster-node --node-id bean2 --root-dir /tmp/multi-bean/bean2 --bootstrap-node-id bean0 --node-peer-addrs 127.0.0.1:11000,127.0.0.1:12000,127.0.0.1:13000 --raft-listen-addr :23000 --node-listen-addr :13000

The above example starts two cluster nodes

- with unique ids bean1 and bean2 which join node bean0 to form a three node cluster.
- the bootstrap node in this case is still `bean0`.


#### Setup beanstalkd proxy

     ./coolbeans beanstalkd  --upstream-addr 127.0.0.1:11000,127.0.0.1:12000:127.0.0.1:13000 --listen-port 11300

The above example starts a new beanstalkd proxy:

- with the proxy upstream pointing to the GRPC services: `127.0.0.1:11000`, `127.0.0.1:12000` and `127.0.0.1:13000`. The proxy automatically detects which of three is the leader and forwards all the requests to the current leader.
- listening for beanstalkd client requests on port `11300`.