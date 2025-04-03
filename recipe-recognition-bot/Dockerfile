# Этап сборки
FROM golang:1.23.4-alpine AS builder

RUN apk add --no-cache git build-base gcc

# Устанавливаем необходимые зависимости
RUN apk add --no-cache git

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем все зависимости
RUN go mod download

COPY . /.env

# Копируем исходный код
COPY . .

# Запускаем SQLC для генерации кода
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
RUN sqlc generate

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot ./cmd/bot

# Этап финальной сборки
FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk --no-cache add ca-certificates tzdata

# Создаем пользователя без привилегий
RUN adduser -D -g '' appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем двоичный файл из этапа сборки
COPY --from=builder /app/bot .

# Копируем миграции базы данных
COPY --from=builder /app/migrations ./migrations

# Устанавливаем владельца файлов
RUN chown -R appuser:appuser /app

# Переключаемся на пользователя без привилегий
USER appuser

# Запускаем приложение
CMD ["./bot"]