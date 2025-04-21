FROM golang:1.24 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc sqlite3 libsqlite3-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -o server main.go

FROM debian:bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y libsqlite3-0 ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server .
COPY static/ ./static/
COPY messages.db .

EXPOSE 8080

CMD ["./server"]

