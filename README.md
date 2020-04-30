<img src="doc/bean_3185124.svg" align=right width=300px />

- [Coolbeans](#coolbeans)
    - [Setup](#setup)
        - [Dependencies](#dependencies)
        - [Building the binary](#building-the-binary)
    - [Example usage](#example-usage)
        - [Configuration](#configuration)
        - [Usage](#usage-1)
    - [Testing](#testing)
        - [Dependencies](#dependencies)
        - [Usage](#usage-2)


Coolbeans
=========

Coolbeans is a lightweight distributed work queue that uses the [beanstalkd protocol](https://github.com/beanstalkd/beanstalkd/blob/master/doc/protocol.txt). 

Unlike a message queue, [beanstalkd](https://github.com/beanstalkd/beanstalkd) provides primitive operations to work with jobs. 

Coolbeans primarily differs from beanstalkd in that it allows the work queue to be replicated across multiple machines.

Setup
-----

This section walks through the process of building the source and running coolbeans.

### Dependencies

Coolbeans is written in golang, it requires go1.13 or newer. It is recommended to use [go version manager](https://github.com/moovweb/gvm) to manage multiple go versions.

A Dockerfile and compose file is provided. This requires Docker version 17 or newer. Refer [docker docs](https://docs.docker.com/) on how to install.

The project depends on protocol buffers:
- Ensure you have protoc & protoc-gen-go installed and accessible in your paths. [This](https://grpc.io/docs/quickstart/go/#protocol-buffers) is an excellent resource which has steps on how to get started. 

### Build & run the binary.

The [Makefile](./Makefile) provides different target options to build and run from source. 

To explore these options: 

    make

Generate a statically linked binary to the local environment:

    make build

Run a single node cluster. Note this spawns two processes, a cluster-node process and beanstalkd proxy.:

    make run-single

Run a three node cluster. Note this spawns four processes, three cluster-node process and beanstalkd proxy.:

    make run-cluster

Releases
--------

Coolbeans is also released as a static binary, which can be downloaded from the [release pages](https://github.com/1xyz/coolbeans/releases)

---

[icon](https://thenounproject.com/term/like/3185124/) by [Llisole](https://thenounproject.com/llisole/) from [the Noun Project](https://thenounproject.com)

