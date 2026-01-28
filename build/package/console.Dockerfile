# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Ставим git, если есть приватные зависимости
RUN apk add --no-cache git

WORKDIR /app

# Сначала копируем только файлы модулей (для кэширования слоев)
COPY go.mod go.sum ./
RUN go mod download

# Теперь копируем весь исходный код
COPY . .

# Собираем именно Console API
RUN CGO_ENABLED=0 GOOS=linux go build -o /console-api cmd/console/main.go

# Stage 2: Final Image
FROM alpine:latest
RUN apk --no-cache add ca-certificates

# Копируем бинарник из билдера
COPY --from=builder /console-api /console-api

# Выставляем порт Console API
EXPOSE 8000

ENTRYPOINT ["/console-api"]
