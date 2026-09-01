// Package testutil spins up a throwaway Postgres for integration tests using
// testcontainers, so `go test ./...` needs Docker but no local Postgres.
package testutil

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ipedrazas/idpad/api/internal/store"
)

// postgresImage pins the same major version the application runs against.
const postgresImage = "postgres:16-alpine"

// DB is a migrated database dedicated to one test package.
type DB struct {
	Pool  *pgxpool.Pool
	Store *store.Store
	URL   string
}

// NewPostgres starts a Postgres container, applies the migrations and returns
// a ready store. The container is terminated when the test (or the TestMain
// that owns it) finishes.
//
// Call it once per package from TestMain and share the result: container
// start-up dominates the runtime, while Reset between tests is cheap.
func NewPostgres(ctx context.Context) (*DB, func(), error) {
	container, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("idpad_test"),
		tcpostgres.WithUsername("idpad"),
		tcpostgres.WithPassword("idpad"),
		// The Postgres entrypoint restarts the server once during init, so the
		// readiness log has to be seen twice before the port is truly usable.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("start postgres container: %w", err)
	}

	terminate := func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			slog.Warn("terminating postgres container", "error", err)
		}
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		terminate()
		return nil, nil, fmt.Errorf("container connection string: %w", err)
	}

	// Container logs are noise in test output; migrations log at info level.
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := store.Migrate(dsn, quiet); err != nil {
		terminate()
		return nil, nil, fmt.Errorf("migrate test database: %w", err)
	}

	pool, err := store.Connect(ctx, dsn, quiet)
	if err != nil {
		terminate()
		return nil, nil, fmt.Errorf("connect to test database: %w", err)
	}

	cleanup := func() {
		pool.Close()
		terminate()
	}

	return &DB{Pool: pool, Store: store.New(pool), URL: dsn}, cleanup, nil
}

// Reset truncates every table so each test starts from a known state. The
// cascade follows the foreign keys, which is exactly the production behaviour.
//
// tags is named explicitly because it is a root table, not a child of ideas:
// truncating ideas clears idea_tags but would leave the vocabulary standing,
// and one test's tags would then be visible to the next.
func (db *DB) Reset(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	if _, err := db.Pool.Exec(ctx, `truncate table ideas, tags restart identity cascade`); err != nil {
		t.Fatalf("reset database: %v", err)
	}
}
