# Build stage
ARG GO_BASE_IMAGE=golang:1.22-alpine
ARG RUNTIME_BASE_IMAGE=alpine:latest
FROM ${GO_BASE_IMAGE} AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux go build -o server ./cmd/server

# Run stage
FROM ${RUNTIME_BASE_IMAGE}
RUN apk add --no-cache ca-certificates sqlite docker-cli

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/admin ./admin

RUN mkdir -p data
VOLUME ["/app/data"]

EXPOSE 8081
CMD ["./server"]
