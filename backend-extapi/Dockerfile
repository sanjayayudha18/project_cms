# Stage 1: Build
FROM golang:1.26.10-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend-extapi/ ./backend-extapi/
# go.work lists backend and backend-exq as workspace modules — Go needs their
# go.mod/go.sum present to resolve the workspace, even though this build never
# imports them.
COPY backend/go.mod backend/go.sum ./backend/
COPY backend-exq/go.mod backend-exq/go.sum ./backend-exq/
WORKDIR /src/backend-extapi
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/extclient ./cmd/extclient

# Stage 2: Run
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/bin/api /app/bin/extclient ./
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8082
ENV TZ=Asia/Jakarta
HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1
CMD ["./api"]
