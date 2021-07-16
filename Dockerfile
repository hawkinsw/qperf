FROM golang:1.16

COPY ./ /go/src/github.com/hawkinsw/qperf
WORKDIR /go/src/github.com/hawkinsw/qperf/server
# TODO: can we build server using `go get` and local sources?
RUN go build .
RUN cp server /go/bin/

ENTRYPOINT ["/go/bin/server"]
