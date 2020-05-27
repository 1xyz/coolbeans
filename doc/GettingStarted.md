Getting Started
===============

A quick way to setup coolbeans on your local machine is to [download the latest binary from the release page](https://github.com/1xyz/coolbeans/releases).

Example (download and unzip for Darwin (macOS)):

    wget https://github.com/1xyz/coolbeans/releases/download/v0.1.10/coolbeans-darwin-amd64-v0.1.10.zip

    tar xvf coolbeans-darwin-amd64-v0.1.10.zip -z


Setup up a single node cluster
------------------------------

The simplest example is to setup a single node cluster. This setup has two processes: (i) coolbeans cluster-node server and (ii) beanstalkd proxy.

#### Start a new coolbeans cluster-node.

    ./coolbeans cluster-node --node-id bean0 --root /tmp/single-bean0 --bootstrap-node-id bean0 --raft-listen-addr :21000 --node-listen-addr :11000


The above example starts a new cluster node:

- with a unique node-id bean0. 
- the cluster-node starts two listeners, the raft service listens on `:21000` and the GRPC service listens on `:11000`. 
- all data logs and snapshots will be persisted under the directory: `/tmp/single-bean0`. 
- the bootstrap node, which is the default assigned leader during the first time cluster formation, is `bean0`.


#### Start a new beanstalkd proxy-server

    ./coolbeans beanstalkd  --upstream-addr 127.0.0.1:11000 --listen-port 11300

The above example starts a new beanstalkd proxy:

- with the proxy upstream pointing to the GRPC service: `127.0.0.1:11000`.
- listening for beanstalkd client requests on port `11300`.


Run a beanstalkd client
-----------------------

Checkout [beanstalkd's community page](https://github.com/beanstalkd/beanstalkd/wiki/Tools) for some tools

Example: We picked yabean since we are familiar with it.

### Download & unzip yabean

Download, and unzip the yabean CLI for the OS/arch https://github.com/1xyz/yabean/releases

    wget https://github.com/1xyz/yabean/releases/download/v0.1.4/yabean_0.1.4_darwin_amd64.tar.gz
    Saving to: ‘yabean_0.1.4_darwin_amd64.tar.gz’

    tar xvf yabean_0.1.4_darwin_amd64.tar.gz
    x LICENSE
    x README.md
    x yabean

### Run a few sample commands

Run some put(s)

    ./yabean put --tube "tube01" --body "hello world"
    c.Put() returned id = 1

    ./yabean put --tube "tube01" --body "你好"
    c.Put() returned id = 2

    ./yabean put --tube "tube01" --body "नमस्ते"
    c.Put() returned id = 3


Reserve a job & delete the reserved job

    ./yabean reserve --tube "tube01"  --string --del
    reserved job id=1 body=11
    body = hello world


Reserve a job and allow a TTL to timeout, the job can be reserved again after ttr

    ./yabean reserve --tube "tube01"  --string
    reserved job id=2 body=6
    body = 你好
    INFO[0000] job allowed to timeout without delete, bury or release actions

Reserve a job & bury the deleted job

    ./yabean reserve --tube "tube01"  --string --bury
    reserved job id=3 body=18
    body = नमते
    buried job 3, pri = 1024


View the tube stats, check out current-jobs-reserved & current-jobs-buried are 1

    ./yabean stats-tube tube01
    StatsTube tube=tube01
    (cmd-delete => 0)
    (cmd-pause-tube => 0)
    (current-jobs-buried => 1)   
    (current-jobs-delayed => 0)
    (current-jobs-ready => 1)
    (current-jobs-reserved => 1)
    (current-jobs-urgent => 0)
    (current-using => 0)
    (current-waiting => 0)
    (current-watching => 0)
    (name => tube01)
    (pause => 0)
    (pause-time-left => 0)
    (total-jobs => 0)


Kick the buried job

    ./yabean kick 3
    job with id = 3 Kicked.

View the tube stats again check out current-jobs-ready is 2, job id 2 moved from reserved to ready (after ttr timeout) and job id 3 got kicked to ready again

    ./yabean stats-tube tube01
    StatsTube tube=tube01
    (cmd-delete => 0)
    (cmd-pause-tube => 0)
    (current-jobs-buried => 0)
    (current-jobs-delayed => 0)
    (current-jobs-ready => 2)
    (current-jobs-reserved => 0)
    (current-jobs-urgent => 0)
    (current-using => 0)
    (current-waiting => 0)
    (current-watching => 0)
    (name => tube01)
    (pause => 0)
    (pause-time-left => 0)
    (total-jobs => 0)

Run a UX program
----------------

I also liked using the [Aurora UI](https://github.com/xuri/aurora)

    wget https://github.com/xuri/aurora/releases/download/2.2/aurora_darwin_amd64_v2.2.tar.gz
    aurora_darwin_amd64_v2.2.tar.gz

    tar xvf aurora_darwin_amd64_v2.2.tar.gz
    x aurora

    ./aurora

- This opens the browser window to http://127.0.0.1:3000/

- Click on the Add Server and add a server at Host = localhost and Port = 11300

- Visit http://127.0.0.1:3000/server?server=localhost:11300 

- Explore further