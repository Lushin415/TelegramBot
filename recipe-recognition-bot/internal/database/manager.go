package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/TelegramBot/recipe-recognition-bot/internal/database/generated"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DBManager управляет соединением с базой данных и предоставляет интерфейс для запросов
type DBManager struct {
	pool    *pgxpool.Pool
	logger  *zap.Logger
	Queries *generated.Queries
}

// NewDBManager создает новый экземпляр DBManager
func NewDBManager(ctx context.Context, connString string, logger *zap.Logger) (*DBManager, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Проверяем соединение
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := generated.New(pool)

	return &DBManager{
		pool:    pool,
		logger:  logger,
		Queries: queries,
	}, nil
}

// Close закрывает соединение с базой данных
func (m *DBManager) Close() {
	m.pool.Close()
}

// RunMigrations запускает миграции базы данных
func (m *DBManager) RunMigrations(migrationsPath string) error {
	conn, err := m.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	driver, err := postgres.WithInstance(conn.Conn(), &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	mg, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"postgres", driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	err = mg.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	m.logger.Info("Database migrations completed successfully")
	return nil
}

// GetUserOrCreate получает пользователя по Telegram ID или создает нового
func (m *DBManager) GetUserOrCreate(ctx context.Context, telegramID int64, username, firstName, lastName string) (*generated.User, error) {
	user, err := m.Queries.GetUserByTelegramID(ctx, telegramID)
	if err == nil {
		// Пользователь найден, проверяем нужно ли обновить данные
		if user.TelegramUsername != username || user.FirstName != firstName || user.LastName != lastName {
			return m.Queries.UpdateUser(ctx, generated.UpdateUserParams{
				TelegramID:       telegramID,
				TelegramUsername: username,
				FirstName:        firstName,
				LastName:         lastName,
			})
		}
		return &user, nil
	}

	// Пользователь не найден, создаем нового
	return m.Queries.CreateUser(ctx, generated.CreateUserParams{
		TelegramID:       telegramID,
		TelegramUsername: username,
		FirstName:        firstName,
		LastName:         lastName,
	})
}
