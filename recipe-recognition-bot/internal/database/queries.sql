-- name: CreateUser :one
INSERT INTO recipe_bot.users (
    telegram_id,
    telegram_username,
    first_name,
    last_name
) VALUES (
             $1, $2, $3, $4
         )
    RETURNING *;

-- name: GetUserByTelegramID :one
SELECT * FROM recipe_bot.users
WHERE telegram_id = $1 LIMIT 1;

-- name: SaveRecipe :one
INSERT INTO recipe_bot.recipes (
    user_id,
    recipe_title,
    recipe_content,
    ingredients
) VALUES (
             $1, $2, $3, $4
         )
    RETURNING *;

-- name: ListUserRecipes :many
SELECT * FROM recipe_bot.recipes
WHERE user_id = $1
ORDER BY created_at DESC
    LIMIT $2;

-- name: GetRecipe :one
SELECT * FROM recipe_bot.recipes
WHERE id = $1 AND user_id = $2 LIMIT 1;

-- name: DeleteRecipe :exec
DELETE FROM recipe_bot.recipes
WHERE id = $1 AND user_id = $2;

-- name: UpdateUser :one
UPDATE recipe_bot.users
SET
    telegram_username = $2,
    first_name = $3,
    last_name = $4,
    updated_at = NOW()
WHERE telegram_id = $1
    RETURNING *;