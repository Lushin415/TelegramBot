# Telegram бот для распознавания продуктов и генерации рецептов

Этот проект представляет собой Telegram бота на Go, который использует компьютерное зрение для распознавания продуктов на фотографиях и генерирует рецепты на основе распознанных продуктов.

## Возможности

- 📷 Распознавание продуктов на фотографиях с помощью OpenAI API
- 🍲 Генерация рецептов на основе распознанных продуктов
- 📝 Сохранение рецептов в базе данных
- 🔍 Просмотр сохраненных рецептов

## Технологии

- **Go** - язык программирования
- **Telegram Bot API** - для взаимодействия с Telegram
- **OpenAI API** - для распознавания изображений и генерации рецептов
- **PostgreSQL** - база данных для хранения пользователей и рецептов
- **Docker** - для контейнеризации и упрощения деплоя

## Установка и запуск

### Предварительные требования

- Go 1.23.4 или выше
- Docker и Docker Compose (для деплоя)
- Telegram Bot Token
- OpenAI API Key

### Локальная разработка

1. Клонировать репозиторий:

```bash
git clone https://github.com/Lushin415/TelegramBot.git
cd recipe-recognition-bot
```

2. Создать файл конфигурации `app.env` на основе образца:

```
TELEGRAM_TOKEN=your_telegram_token_here
OPENAI_API_KEY=your_openai_api_key_here
POSTGRES_URI=postgres://username:password@localhost:5432/recipebot
LOG_LEVEL=info
APP_ENVIRONMENT=development
MAX_RECIPES_PER_USER=50
```

3. Установить зависимости:

```bash
go mod download
```

4. Запустить PostgreSQL (например, через Docker):

```bash
docker-compose -f docker-compose.db.yml up -d
```

5. Запустить бота:

```bash
go run cmd/bot/main.go
```

### Запуск через Docker Compose

1. Создать файл `.env` с переменными окружения:

```
TELEGRAM_TOKEN=YOUR_KEY_TELEGRAM
OPENAI_API_KEY=YOUR_KEY_OPENAI
POSTGRES_URI=postgres://recipebot:recipebot_password@localhost:5432/recipebot
LOG_LEVEL=info
APP_ENVIRONMENT=production
MAX_RECIPES_PER_USER=50
```

2. Запустить с помощью Docker Compose:

```bash
docker-compose up -d
```

## Использование

1. Найдите бота в Telegram по его имени
2. Отправьте команду `/start` для начала работы
3. Отправьте фотографию продуктов
4. Бот распознает продукты и предложит рецепт
5. Используйте команду `/recipes` для просмотра сохраненных рецептов

## Структура проекта

```
recipe-recognition-bot/
├── cmd/
│   └── bot/             - Точка входа для приложения
├── internal/
│   ├── bot/             - Логика Telegram бота
│   ├── config/          - Управление конфигурацией
│   ├── database/        - Работа с базой данных
│   │   └── generated/   - Код, сгенерированный SQLC
│   ├── recipes/         - Генерация рецептов
│   └── vision/          - Распознавание продуктов
├── migrations/          - Миграции базы данных
├── docker-compose.yml   - Конфигурация Docker Compose
├── Dockerfile           - Инструкции для сборки Docker образа
└── README.md            - Документация проекта
```

## Разработка

### Генерация кода для базы данных

Для обновления сгенерированного кода после изменения схемы базы данных:

```bash
sqlc generate
```