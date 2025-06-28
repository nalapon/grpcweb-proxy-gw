FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s" \
  -o /app/proxy \
  ./cmd/proxy
FROM alpine:latest
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/proxy /app/proxy
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8088
ENTRYPOINT ["/app/proxy"]
CMD ["serve"]
