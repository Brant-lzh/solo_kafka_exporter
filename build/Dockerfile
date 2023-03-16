FROM golang:1.19-alpine3.17 AS builder

MAINTAINER brant

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN export GOPROXY=https://goproxy.cn,direct && GO111MODULE=on go mod download

COPY . .

WORKDIR /app/cmd/all_in_one/

RUN GOOS=linux GOFLAGS=-buildvcs=false GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o solo_kafka_exporter .


# runner
FROM alpine AS runner

COPY --from=builder /app/cmd/all_in_one/solo_kafka_exporter /solo_kafka_exporter

WORKDIR /

ENTRYPOINT [ "/solo_kafka_exporter"]
