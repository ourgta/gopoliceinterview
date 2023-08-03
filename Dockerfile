FROM golang:alpine3.18 AS build

WORKDIR /usr/src/app

COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd
COPY internal ./internal

RUN go mod download

RUN go build -ldflags="-s -w" -o /usr/local/bin/app cmd/gopoliceinterview/main.go

FROM alpine:3.18

COPY --from=build /usr/local/bin/app /app

CMD ["/app"]
