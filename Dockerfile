FROM golang:1-stretch as builder
COPY . /build
WORKDIR /build
RUN GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOFLAGS=-mod=vendor go build -o vault

FROM ubuntu
WORKDIR /usr/local/bin
RUN apt-get update; apt-get install -y ca-certificates
COPY --from=builder /build/vault .
CMD ["vault"]