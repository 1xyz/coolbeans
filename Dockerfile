FROM golang:1.16.3-alpine AS builder

RUN apk update && apk add make git build-base curl protobuf && \
     rm -rf /var/cache/apk/*

RUN go get golang.org/x/tools/cmd/stringer

ADD . /go/src/github.com/1xyz/coolbeans
WORKDIR /go/src/github.com/1xyz/coolbeans
RUN make release/linux

###

FROM alpine:latest AS coolbeans  

RUN apk update && apk add ca-certificates bash
WORKDIR /root/
COPY --from=builder /go/src/github.com/1xyz/coolbeans/bin/linux/coolbeans .
