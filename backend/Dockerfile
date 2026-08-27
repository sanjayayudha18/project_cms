# Stage 1: Build
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend/ ./backend/
# go.work lists backend-cit as a workspace module — Go needs its go.mod/go.sum
# present to resolve the workspace, even though this build never imports it.
COPY backend-cit/go.mod backend-cit/go.sum ./backend-cit/
WORKDIR /src/backend
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api

# Stage 2: Run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/bin/api .
COPY --from=builder /src/backend/migrations ./migrations
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
ENV TZ=Asia/Jakarta
HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
CMD ["./api"]
