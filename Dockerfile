# syntax=docker/dockerfile:1
FROM golang:1.19-alpine


WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
COPY ./static ./static/

RUN go build -o ./opensky-tracker

EXPOSE 8081

CMD ["./opensky-tracker"]