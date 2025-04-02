package database

import (
	"context"
	"errors"
	"github.com/TelegramBot/recipe-recognition-bot/internal/database/generated"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

type DBManager struct {
	pool    *pgxpool.Pool
	logger  *zap.Logger
	Queries *database.Queries
}

func NewDBManager(ctx context.Context, connString string, logger *zap.Logger) (*DBManager, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &DBManager{
		pool:    pool,
		logger:  logger,
		Queries: database.New(pool),
	}, nil
}

func (m *DBManager) Close() {
	m.pool.Close()
}

func (m *DBManager) RunMigrations(migrationsPath string) error {
	// Получаем конфигурацию пула
	poolConfig := m.pool.Config()

	// Создаем *sql.DB используя ConnConfig
	db := stdlib.OpenDB(*poolConfig.ConnConfig)
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	mg, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"postgres", driver,
	)
	if err != nil {
		return err
	}

	err = mg.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func (m *DBManager) GetUserOrCreate(ctx context.Context, telegramID int64, username, firstName, lastName string) (*database.RecipeBotUser, error) {
	user, err := m.Queries.GetUserByTelegramID(ctx, telegramID)
	if err == nil {
		return &user, nil
	}

	// Пользователь не найден, создаем нового
	usernameText := pgtype.Text{String: username, Valid: username != ""}
	firstNameText := pgtype.Text{String: firstName, Valid: firstName != ""}
	lastNameText := pgtype.Text{String: lastName, Valid: lastName != ""}

	user, err = m.Queries.CreateUser(ctx, database.CreateUserParams{
		TelegramID:       telegramID,
		TelegramUsername: usernameText,
		FirstName:        firstNameText,
		LastName:         lastNameText,
	})

	return &user, err
}
