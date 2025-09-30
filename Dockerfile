FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o vault-secrets-exporter ./cmd/exporter
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/vault-secrets-exporter .

EXPOSE 9102
ENTRYPOINT ["./vault-secrets-exporter"]
