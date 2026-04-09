FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /service ./services/delivery-service/cmd

FROM alpine:3.22 AS runner
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /service .
COPY --from=builder /app/services/delivery-service/migrations ./migrations
EXPOSE 8080 9090
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["./service"]
