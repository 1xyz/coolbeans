# Coolbeans

<img src="./doc/bean_3185124.svg" align="right" height="200" width="200" >

Coolbeans is a lightweight distributed work queue based that uses the beanstalkd protocol.


## Getting Started

Coolbeans is  compiled as single statically linked binary, that can be obtained from the [releases]() page.

### build and run the binary

The [Makefile](./Makefile) provides different target options to build and run from source. To explore these options, run `make`

Example.

```
# Generate a statically linked binary to the local environment
$ make build
..

# Run a single node cluster. Note this spawns two processes, a cluster-node process and beanstalkd proxy.
$ make run-single
...
go get github.com/mattn/goreman
goreman -f Procfiles/dev.procfile start

```

---

[icon](https://thenounproject.com/term/like/3185124/) by [Llisole](https://thenounproject.com/llisole/) from [the Noun Project](https://thenounproject.com)
