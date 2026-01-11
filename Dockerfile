# Build
FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o threads-connector ./cmd/server/main.go

# Runtime
FROM alpine:3.16
WORKDIR /app
COPY --from=builder /app/threads-connector .
COPY .env /app/.env
EXPOSE ${PORT}
CMD ["./threads-connector"]