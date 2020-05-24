![ci-build](https://github.com/1xyz/coolbeans/workflows/ci-build/badge.svg)
![ci-release](https://github.com/1xyz/coolbeans/workflows/ci-release/badge.svg)

<img src="doc/bean_3185124.svg" align=right width=200px />

- [Coolbeans](#coolbeans)
    - [Motivation](#motivation)
    - [Key features](#key-features)
    - [Releases](#releases)
    - [Getting started](#getting-started)
    - [How to contribute](#how-to-contribute)
    - [Local Development](doc/Developer.md)


Coolbeans
=========

Coolbeans is a distributed replicated work queue service that implements the [beanstalkd protocol](https://github.com/beanstalkd/beanstalkd/blob/master/doc/protocol.txt). 

Unlike a message queue, [beanstalkd](https://github.com/beanstalkd/beanstalkd) is a work queue that provides primitive operations to work with jobs. 

Coolbeans primarily differs from beanstalkd in that it allows the work queue to be replicated across multiple machines. It uses the [RAFT consensus algorithm](https://raft.github.io/) to replicate the job state consistently across machines.

Motivation
----------

Beanstalkd is a [feature-rich](https://www.igvita.com/2010/05/20/scalable-work-queues-with-beanstalk/) and easy to use queue. Beanstalkd, however has a few drawbacks that include: (i) A lack of replication or high availability in terms of machine failures. (ii) There is no native sharding, (iii) No native support for encryption & authentication between the service & the client.

Given the initial setup of beanstalkd is simple, having a HA or sharded production setup is non-trivial. Our premise with Coolbeans is to provide a replicated beanstalkd queue followed by addressing the other issues incrementally. Read about our design approach [here](doc/Design.md).

Key features
------------

- A fully replicated work queue built using [Hashicorp's Raft library](https://github.com/hashicorp/raft).
- Strong consistency of all queue operations. 
- Compatible with [existing beanstalkd clients](https://github.com/beanstalkd/beanstalkd/wiki/Client-Libraries).
- Easy installation, available as a static binary or as a Linux docker image.
- Monitor metrics using Prometheus and visualize them via Grafana.


Releases
--------

- Static binary can be downloaded from the [release pages](https://github.com/1xyz/coolbeans/releases)
- Docker image  can be pulled from [here](https://hub.docker.com/r/1xyz/coolbeans)


Getting Started 
---------------

Refer the [Getting started guide](doc/Setup.md)  

If you prefer to try to run coolbeans on Kubernetes, refer [here](https://github.com/1xyz/coolbeans-k8s).


How to contribute
-----------------

Coolbeans is currently at `alpha` release quality. It is all about improving the quality of this by testing, testing & more testing.

Here are a few ways you can contribute:

- Be an early adopter, Try it out on your machine, testbed or pre-production stack and give us [feedback or report issues](https://github.com/1xyz/coolbeans/issues/new/choose).

- Have a feature in mind. Tell us more about by [filing an issue](https://github.com/1xyz/coolbeans/issues/new/choose).

- Want to contribute to code, documentation. Checkout the [contribution guide](./CONTRIBUTING.md). 

---

[icon](https://thenounproject.com/term/like/3185124/) by [Llisole](https://thenounproject.com/llisole/) from [the Noun Project](https://thenounproject.com)

