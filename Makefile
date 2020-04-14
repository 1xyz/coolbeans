GO=go
GOFMT=gofmt
GOREMAN=goreman
PROTOC=protoc
DELETE=rm
DOCKER=docker
DOCKER_COMPOSE=docker-compose
BINARY=coolbeans
BUILD_BINARY=bin/$(BINARY)
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
# current git version short-hash
VER = $(shell git rev-parse --short HEAD)
TAG = "$(BINARY):$(VER)"

info:
	@echo " target         ⾖ Description.                                    "
	@echo " ----------------------------------------------------------------- "
	@echo
	@echo " build          generate a local build ⇨ $(BUILD_BINARY)          "
	@echo " clean          clean up bin/ & go test cache                      "
	@echo " fmt            format go code files using go fmt                  "
	@echo " generate       generate enum-strings & test-fakes go files        "
	@echo " protoc         compile proto files to generate go files           "
	@echo " release/darwin generate a darwin target build                     "
	@echo " release/linux  generate a linux target build                      "
	@echo " tidy           clean up go module file                            "
	@echo
	@echo " Run targets                                                       "
	@echo " -----------"
	@echo " run-single     run a single node cluster w/ beanstalkd proxy      "
	@echo " run-cluster    run a three node cluster w/ beanstalkd proxy       "
	@echo " run-beanstalkd run a single process beanstalkd                    "
	@echo
	@echo " Test targets                                                      "
	@echo " -----------"
	@echo " test       run unit-tests                                         "
	@echo " testc      run unit-tests w/ coverage                             "
	@echo " testv      run unit-tests verbose                                 "
	@echo " test-int   run integration-tests requires a running beanstalkd    "
	@echo
	@echo " Docker targets"
	@echo " --------------"
	@echo " docker-build        build the docker image $(TAG)                 "
	@echo " docker-compose-up   run docker-compose-up                         "
	@echo " docker-compose-down run docker-compose-down                       "
	@echo " ------------------------------------------------------------------"

build: clean fmt protoc
	$(GO) build -o $(BUILD_BINARY) -v main.go


.PHONY: clean
clean:
	$(DELETE) -rf bin/
	$(GO) clean -cache


.PHONY: fmt
fmt:
	$(GOFMT) -l -w $(SRC)


# tools deps to generate code (stringer...)
# https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
.PHONY: generate
generate:
	$(GO) generate ./...


.PHONY: protoc
protoc:
	$(GO) get -u github.com/golang/protobuf/protoc-gen-go
	$(PROTOC) -I api/v1 api/v1/*.proto --go_out=plugins=grpc:api/v1 --go_opt=paths=source_relative


release/%: clean fmt protoc
	@echo "build no race on alpine. https://github.com/golang/go/issues/14481"
	$(GO) test ./...
	@echo "build GOOS: $(subst release/,,$@) & GOARCH: amd64"
	GOOS=$(subst release/,,$@) GOARCH=amd64 $(GO) build -o bin/$(subst release/,,$@)/$(BINARY) -v main.go


.PHONY: run-single
run-single: build
	$(GO) get github.com/mattn/goreman
	$(GOREMAN) -f Procfiles/dev.procfile start

.PHONY: run-cluster
run-cluster: build
	$(GO) get github.com/mattn/goreman
	$(GOREMAN) -f Procfiles/dev-cluster.procfile start

.PHONY: run-beanstalkd
run-beanstalkd: build
	$(GO) get github.com/mattn/goreman
	$(GOREMAN) -f Procfiles/beanstalkd.procfile start


# test w/ race detector on always
# https://golang.org/doc/articles/race_detector.html#Typical_Data_Races
.PHONY: test
test: build
	$(GO) test -race ./...


.PHONY: testv
testv: build
	$(GO) test -v -race ./...


.PHONY: testc 
testc: build
	$(GO) get github.com/ory/go-acc
	./coverage_test.sh 
	$(GO) tool cover -html=coverage.txt

.PHONY: test-int
test-int: build
	$(GO) test -v -tags=integration ./integration

.PHONY: tidy
tidy:
	$(GO) mod tidy

docker-build:
	$(DOCKER) build -t $(TAG) -f Dockerfile .

docker-compose-up:
	$(DOCKER_COMPOSE) --file compose/docker-compose.yml --project-directory . up

docker-compose-down:
	$(DOCKER_COMPOSE) --file compose/docker-compose.yml --project-directory . down
