# syntax=docker/dockerfile:1
#build image
FROM golang:1.19-alpine as build

ENV CGO_ENABLED 0
ENV GOOS linux

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
#COPY *.go ./
#COPY ./web ./web/
#COPY ./notificator ./notificator/
#COPY ./db ./db/
#COPY ./cmd/server ./cmd/server/

#COPY ./static ./static/
#RUN cd ./cmd/server/
RUN go build -ldflags="-s -w" -o /app/opensky-tracker-server ./cmd/server/


# make second small executable container
FROM alpine:latest as server

WORKDIR /app

COPY --from=build /app/opensky-tracker-server ./opensky-tracker-server

RUN chmod +x ./opensky-tracker-server

EXPOSE 8081

CMD ["./opensky-tracker-server"]