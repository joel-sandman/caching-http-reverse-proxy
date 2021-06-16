FROM golang:latest AS build-env
# Build stand-alone (independent of libc implementation)
ENV CGO_ENABLED 0
ADD . /go/src/github.com/joel-sandman/caching-http-reverse-proxy
WORKDIR /go/src/github.com/joel-sandman/caching-http-reverse-proxy
RUN go build -mod=vendor -o /caching-http-reverse-proxy
# Multi-stage!
FROM alpine
WORKDIR /
COPY --from=build-env /caching-http-reverse-proxy /usr/local/bin/
ENTRYPOINT caching-http-reverse-proxy