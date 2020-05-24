Contributing
============

Coolbeans is currently at `alpha` release quality. It is all about improving the quality by adoption and testing.

By participating in this project you agree to abide by the [code of conduct](https://www.contributor-covenant.org/version/2/0/code_of_conduct/). 

- [Building coolbeans](#building-coolbeans)
    - [Dependencies](#dependencies)
    - [Build the binary](#build-the-binary)
    - [Run the service](#run-the-service)
    - [Other run options](#other-run-options)
- [Testing](#testing)
    - [Setup a beanstalkd client to test manually](#manual-test)
    - [Unit Tests](#unit-tests)
    - [End to end Tests](#run-end-to-end-tests)
- [Other](#other)


Building coolbeans
------------------

This section walks through the process of building the source and running coolbeans.

### Dependencies

* [Install golang v1.13+](https://golang.org/dl/)
    - Coolbeans is written in golang, it requires go version 1.13 or newer. I prefer to use [go version manager](https://github.com/moovweb/gvm) to manage multiple go versions. 
    - Ensure `$GOPATH/bin` is added to your path.

* [Install Docker](https://docs.docker.com/get-docker/)
    - A [Dockerfile](../Dockerfile) is provided. 

* [Install Protocol Buffer Compiler (protoc) & the Go plugin (protoc-gen-go)](https://grpc.io/docs/quickstart/go/#protocol-buffers)
    - The project depends on protocol buffers and uses the Grpc library. 
    - Ensure you have `protoc` & `protoc-gen-go` installed and accessible in your paths.

### Build the binary.

The [Makefile](./Makefile) provides different target options to build and run from source. 

To explore these options, run `make` which shows all possible targets:

    make

For example: To generate a statically linked binary for the local operating-system.

    make build


### Run the service

Coolbeans typically runs as a two processes, refer the [design](doc/Design.md) for more detail.

Run a single node cluster. Note this creates two processes, a cluster-node process and beanstalkd proxy:

    make run-single


Run a three node cluster. Note this spawns four processes, three cluster-node processes and beanstalkd proxy.:

    make run-cluster


### Other Run options

Run a single process beanstalkd (no replication via Raft, the entire queue is in memory):

    make run-beanstalkd

Run a three node cluster via docker-compose. Run this prior to running docker-compose-up

    make docker-compose-build

    make docker-compose-up

Once done:

    make docker-compose-down


Testing
-------

### Manual test

Download and run a beanstalk client from [here](https://github.com/beanstalkd/beanstalkd/wiki/Tools).

Some client I tested with: 
- [Aurora](https://github.com/xuri/aurora/releases/tag/2.2) 
- [yabean](https://github.com/1xyz/yabean)


### Unit Tests

Run unit-tests

    make test

Explore other test options by running `make`


### Run end to end tests

Run an end to end test scenarios against a running cluster.

    make test-e2e


Other
-----

- Reporting an issue, please refer [here](https://github.com/1xyz/coolbeans/issues/new/choose)

- Guidelines for a good commit message. please refer [here](https://golang.org/doc/contribute.html#commit_messages)
