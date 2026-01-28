FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /uag cmd/uag/main.go

FROM alpine:latest
# Устанавливаем сертификаты для работы с внешними API через TLS
RUN apk --no-cache add ca-certificates
COPY --from=builder /uag /uag
ENTRYPOINT ["/uag"]
