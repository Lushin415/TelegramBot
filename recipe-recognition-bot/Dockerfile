# Этап сборки
FROM golang:1.23.4-alpine AS builder

RUN apk add --no-cache git build-base gcc

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем env-файл
COPY .env .env

# Копируем исходный код
COPY . .

# Запускаем SQLC для генерации кода
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
RUN sqlc generate

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot ./cmd/bot

# Этап финальной сборки
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
RUN adduser -D -g '' appuser

WORKDIR /app

# Копируем бинарник и миграции
COPY --from=builder /app/bot .
COPY --from=builder /app/migrations ./migrations

# Копируем .env (на финальном этапе!)
COPY --from=builder /app/.env .env

RUN chown -R appuser:appuser /app
USER appuser

CMD ["./bot"]
