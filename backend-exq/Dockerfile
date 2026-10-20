# -- Build stage --
FROM golang:1.26.9-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend-exq/ ./backend-exq/
# go.work lists backend as a workspace module — Go needs its go.mod/go.sum
# present to resolve the workspace, even though this build never imports it.
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend-exq
RUN go build -o /app/backend-exq ./cmd/api

# -- Runtime stage --
FROM alpine:3.24
RUN apk add --no-cache wget
COPY --from=builder /app/backend-exq /usr/local/bin/backend-exq
ENTRYPOINT ["backend-exq"]
