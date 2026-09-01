// Package store is the repository layer: every SQL statement the service runs
// lives here, behind context-aware methods returning domain types.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors the HTTP layer maps onto status codes.
var (
	// ErrNotFound means the addressed row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the write violated a uniqueness constraint.
	ErrConflict = errors.New("conflict")
)

// Store owns the connection pool and exposes the repository methods.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool. The pool's lifetime is the caller's.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pool, used for health checks.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// newID returns a UUIDv7. Generating IDs in the application keeps them
// time-orderable without depending on a database extension.
func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuidv7: %w", err)
	}
	return id.String(), nil
}

// isUniqueViolation reports whether err is a Postgres unique constraint error.
func isUniqueViolation(err error) bool {
	return hasPGCode(err, "23505")
}

// isForeignKeyViolation reports whether err is a Postgres foreign key error,
// which is how a write naming a row that does not exist fails.
func isForeignKeyViolation(err error) bool {
	return hasPGCode(err, "23503")
}

// hasPGCode reports whether err is a Postgres error carrying SQLSTATE code.
func hasPGCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
