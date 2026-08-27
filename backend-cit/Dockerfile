# -- Build stage --
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend-cit/ ./backend-cit/
WORKDIR /src/backend-cit
RUN go build -o /app/backend-cit ./cmd/api

# -- Runtime stage --
FROM alpine:3.20
RUN apk add --no-cache wget
COPY --from=builder /app/backend-cit /usr/local/bin/backend-cit
ENTRYPOINT ["backend-cit"]
