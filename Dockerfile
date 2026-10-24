# Build stage
FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/icarosu ./cmd/bot

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/icarosu /app/icarosu
COPY configs /app/configs
RUN mkdir -p /app/data && chown -R app:app /app/data
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -f http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/icarosu"]
CMD ["-config", "/app/configs/config.yaml"]