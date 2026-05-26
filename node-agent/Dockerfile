FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/fblink-node-agent ./cmd/fblink-node-agent

FROM alpine:3.20
RUN apk add --no-cache docker-cli ca-certificates curl
COPY --from=build /out/fblink-node-agent /usr/local/bin/fblink-node-agent
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/fblink-node-agent"]
