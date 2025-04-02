// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package database

import (
	"context"
)

type Querier interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (RecipeBotUser, error)
	DeleteRecipe(ctx context.Context, arg DeleteRecipeParams) error
	GetRecipe(ctx context.Context, arg GetRecipeParams) (RecipeBotRecipe, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (RecipeBotUser, error)
	ListUserRecipes(ctx context.Context, arg ListUserRecipesParams) ([]RecipeBotRecipe, error)
	SaveRecipe(ctx context.Context, arg SaveRecipeParams) (RecipeBotRecipe, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (RecipeBotUser, error)
}

var _ Querier = (*Queries)(nil)
