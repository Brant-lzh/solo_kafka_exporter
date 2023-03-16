FROM golang:1.19-alpine3.17 AS builder

MAINTAINER brant

WORKDIR /app

COPY . ./

ENV GO111MODULE=on


RUN go mod tidy

RUN go mod download



WORKDIR /app/cmd/all_in_one/

ENV GOPROXY https://goproxy.cn,direct

RUN GOOS=linux GOFLAGS=-buildvcs=false GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o solo_kafka_exporter .


# runner
FROM alpine AS runner

COPY --from=builder /app/cmd/all_in_one/solo_kafka_exporter /solo_kafka_exporter

WORKDIR /

ENTRYPOINT [ "/solo_kafka_exporter"]









#FROM alpine AS builder
#
## Install the Certificate-Authority certificates for the app to be able to make
## calls to HTTPS endpoints.
## Git is required for fetching the dependencies.
#RUN apk add --no-cache ca-certificates
#
## Final stage: the running container.
#FROM scratch AS final
#
## Add maintainer label in case somebody has questions.
#LABEL maintainer="Kris.Budde@gmail.com"
#
## Import the Certificate-Authority certificates for enabling HTTPS.
#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
#
## Import the compiled executable from the first stage.
#COPY rabbitmq_exporter /rabbitmq_exporter
#
## Declare the port on which the webserver will be exposed.
## As we're going to run the executable as an unprivileged user, we can't bind
## to ports below 1024.
#EXPOSE 9419
#
## Perform any further action as an unprivileged user.
#USER 65535:65535
#
## Check if exporter is alive; 10 retries gives prometheus some time to retrieve bad data (5 minutes)
#HEALTHCHECK --retries=10 CMD ["/rabbitmq_exporter", "-check-url", "http://localhost:9419/health"]
#
## Run the compiled binary.
#ENTRYPOINT ["/rabbitmq_exporter"]
