-- Создание схемы
CREATE SCHEMA IF NOT EXISTS recipe_bot;

-- Создание таблицы пользователей
CREATE TABLE IF NOT EXISTS recipe_bot.users (
                                                id SERIAL PRIMARY KEY,
                                                telegram_id BIGINT NOT NULL UNIQUE,
                                                telegram_username TEXT,
                                                first_name TEXT,
                                                last_name TEXT,
                                                created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Создание таблицы рецептов
CREATE TABLE IF NOT EXISTS recipe_bot.recipes (
                                                  id SERIAL PRIMARY KEY,
                                                  user_id INT NOT NULL REFERENCES recipe_bot.users(id) ON DELETE CASCADE,
    recipe_title TEXT NOT NULL,
    recipe_content TEXT NOT NULL,
    ingredients JSONB, -- хранение списка ингредиентов в JSON формате
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES recipe_bot.users(id)
    );

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON recipe_bot.users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_recipes_user_id ON recipe_bot.recipes(user_id);
CREATE INDEX IF NOT EXISTS idx_recipes_created_at ON recipe_bot.recipes(created_at);