Getting started - Three node cluster
====================================

In the [Getting started page](./GettingStarted.md), we setup a single node cluster. Here, we setup a three node node cluster, which is tolerant to a single node failure. We call this setup highly available (HA).

Setup cluster
-------------

Following is a step-by-step guide to getting this HA cluster up and running. This setup has three cluster node processes and one beanstalkd proxy process. You can see a more production-like setup for kubernetes [here](https://github.com/1xyz/coolbeans-k8s)


#### Setup cluster-node bean0


    # create a directory for demonstration purposes.
    $ mkdir -p /tmp/ha

    $ ./coolbeans --quiet cluster-node --node-id bean0 --root-dir /tmp/ha/bean0 --bootstrap-node-id bean0 --node-peer-addrs=:11000,:12000,:13000 --raft-listen-addr=127.0.0.1:21000 --node-listen-addr=127.0.0.1:11000 --raft-advertized-addr=127.0.0.1:21000 --prometheus-addr=127.0.0.1:2020

The above example starts a new cluster node

- with a unique node-id `bean0`. 
- the cluster-node starts two listeners, the raft service listens on `127.0.0.1:21000` and the GRPC service listens on `:11000`.
- the raft advertised address indicates the address the other raft peers will use to connect.
- all data logs and snapshots will be persisted under the directory: `/tmp/ha/bean0`. 
- the bootstrap node, which the default assigned leader during the first time cluster formation, is `bean0`.
- additionally, we define all the initial peers: `:11000`,`:12000`,`:13000`.


#### Setup cluster-nodes bean1 & bean2

    ./coolbeans --quiet cluster-node --node-id bean1 --root-dir /tmp/ha/bean1 --bootstrap-node-id bean0 --node-peer-addrs=:11000,:12000,:13000 --raft-listen-addr=127.0.0.1:22000 --node-listen-addr=127.0.0.1:12000 --raft-advertized-addr=127.0.0.1:22000 --prometheus-addr=127.0.0.1:2021

    ./coolbeans --quiet cluster-node --node-id bean2 --root-dir /tmp/ha/bean2 --bootstrap-node-id bean0 --node-peer-addrs=:11000,:12000,:13000 --raft-listen-addr=127.0.0.1:23000 --node-listen-addr=127.0.0.1:13000 --raft-advertized-addr=127.0.0.1:23000 --prometheus-addr=127.0.0.1:2022 

The above example starts two cluster nodes

- with unique ids `bean1` and `bean2` which join node `bean0` to form a three node cluster.
- the bootstrap node in this case is still `bean0`. What that means, is if we bring up all the three nodes in parallel, only bean0 becomes the leader, and bean1 and bean2 become followers.


#### Setup beanstalkd proxy

     ./coolbeans --quiet beanstalkd --listen-addr 127.0.0.1 --upstream-addrs 127.0.0.1:11000,127.0.0.1:12000,127.0.0.1:13000 --listen-port 11300 >> /tmp/coolbeans-sidecar.log 2>> /tmp/coolbeans-sidecar_error.log

The above example starts a new beanstalkd proxy:

- with the proxy upstream pointing to the GRPC services: `127.0.0.1:11000`, `127.0.0.1:12000` and `127.0.0.1:13000`. The proxy automatically detects which of three is the leader and forwards all the requests to the current leader.
- listening for beanstalkd client requests on port `11300`.

#### Query to find out which node is the elected leader

```
$  ./coolbeans cluster-client is_leader --node-addr 127.0.0.1:11000
isNodeLeader: false

$ ./coolbeans cluster-client is_leader --node-addr 127.0.0.1:12000
isNodeLeader: false

$ ./coolbeans cluster-client is_leader --node-addr 127.0.0.1:13000
isNodeLeader: true
```

From the above, you can see that the node `bean2` was elected the leader even though the boostrapped leader initially was `bean0`.