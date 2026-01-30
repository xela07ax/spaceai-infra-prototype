package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/xela07ax/spaceai-infra-prototype/internal/domain"
)

func (r *AgentRepo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, email, username, password_hash, role, scopes, created_at, updated_at 
		FROM users WHERE username = $1`

	u := &domain.User{}
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role, &u.Scopes, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}
