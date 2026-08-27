# -- Build stage --
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend-cit/ ./backend-cit/
# go.work lists backend as a workspace module — Go needs its go.mod/go.sum
# present to resolve the workspace, even though this build never imports it.
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend-cit
RUN go build -o /app/backend-cit ./cmd/api

# -- Runtime stage --
FROM alpine:3.20
RUN apk add --no-cache wget
COPY --from=builder /app/backend-cit /usr/local/bin/backend-cit
ENTRYPOINT ["backend-cit"]
