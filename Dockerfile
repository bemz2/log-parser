FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/topology-parser ./cmd/topology-parser

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /bin/topology-parser /app/topology-parser
COPY migrations /app/migrations
COPY data /app/data

ENV PORT=8080
ENV DATA_DIR=data
ENV MIGRATIONS_DIR=migrations

EXPOSE 8080

CMD ["/app/topology-parser"]
