package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: manage up|status|down (down only for disposable test databases)")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("database configuration invalid")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "up":
		results, e := provider.Up(ctx)
		if e != nil {
			return e
		}
		fmt.Printf("Applied %d migrations.\n", len(results))
		return nil
	case "status":
		statuses, e := provider.Status(ctx)
		if e != nil {
			return e
		}
		for _, status := range statuses {
			fmt.Printf("%v\n", status)
		}
		return nil
	case "down":
		var name string
		if e := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); e != nil {
			return e
		}
		if os.Getenv("MIGRATION_ALLOW_DOWN") != "true" || !strings.HasPrefix(name, "forma_test_") {
			return fmt.Errorf("down requires MIGRATION_ALLOW_DOWN=true and a disposable forma_test_* database")
		}
		_, err = provider.Down(ctx)
		return err
	default:
		return fmt.Errorf("unknown command")
	}
}
