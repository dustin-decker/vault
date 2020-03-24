FROM golang:1.13.5-stretch as builder
COPY . /build
WORKDIR /build
ENV GO111MODULE=on
RUN CGO_ENABLED=1 GOOS=linux GOFLAGS=-mod=vendor go build -o vault

FROM ubuntu
WORKDIR /usr/local/bin
COPY --from=builder /build/vault .
CMD ["vault"]