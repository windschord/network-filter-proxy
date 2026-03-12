FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filter-proxy ./cmd/filter-proxy

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /filter-proxy /filter-proxy
# Port 3128: proxy listener (all interfaces)
# Port 8080: management API (configurable via API_BIND_ADDR, default 127.0.0.1)
EXPOSE 3128 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=3 \
  CMD ["/filter-proxy", "healthcheck"]
ENTRYPOINT ["/filter-proxy"]
