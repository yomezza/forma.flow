package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/yomezza/forma.flow/backend/internal/auth"
	"github.com/yomezza/forma.flow/backend/internal/config"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"net/mail"
	"os"
	"strings"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	email := os.Getenv("BOOTSTRAP_EMAIL")
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email {
		return fmt.Errorf("BOOTSTRAP_EMAIL required")
	}
	fmt.Fprintln(os.Stderr, "Read owner password from standard input (not echoed by this command; use a pipe or protected file).")
	p, e := bufio.NewReader(os.Stdin).ReadString('\n')
	if e != nil {
		return fmt.Errorf("password input required")
	}
	p = strings.TrimSuffix(p, "\n")
	if len(p) < 12 || len(p) > 1024 {
		return fmt.Errorf("password must be 12..1024 bytes")
	}
	hash, e := auth.Hash(p)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, e := postgres.Open(ctx, c.DatabaseURL, c.PoolMax)
	if e != nil {
		return fmt.Errorf("database connection failed")
	}
	defer s.Pool.Close()
	if e = s.CheckSchema(ctx); e != nil {
		return e
	}
	return postgres.Bootstrap(ctx, s, c.StudioTimeZone, email, hash)
}
