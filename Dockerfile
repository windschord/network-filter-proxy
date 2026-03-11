FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filter-proxy ./cmd/filter-proxy

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /filter-proxy /filter-proxy
EXPOSE 3128 8080
ENTRYPOINT ["/filter-proxy"]
