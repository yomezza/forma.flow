package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
	_ "time/tzdata"
)

type Config struct {
	DatabaseURL, HTTPAddr, PublicOrigin, StudioTimeZone string
	CookieSecure                                        bool
	SolverNodes, MaxComponent, PoolMax                  int
	SessionTTL                                          time.Duration
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), HTTPAddr: env("HTTP_ADDR", ":8080"), PublicOrigin: os.Getenv("PUBLIC_ORIGIN"), StudioTimeZone: os.Getenv("STUDIO_TIME_ZONE"), SessionTTL: 12 * time.Hour}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	u, err := url.Parse(c.PublicOrigin)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.Path != "" || u.RawQuery != "" || u.User != nil || u.Fragment != "" {
		return c, fmt.Errorf("PUBLIC_ORIGIN must be an HTTP(S) origin")
	}
	if c.StudioTimeZone == "" {
		return c, fmt.Errorf("STUDIO_TIME_ZONE must be explicitly configured")
	}
	if _, err = time.LoadLocation(c.StudioTimeZone); err != nil {
		return c, fmt.Errorf("invalid STUDIO_TIME_ZONE")
	}
	c.CookieSecure, err = strconv.ParseBool(env("COOKIE_SECURE", "true"))
	if err != nil {
		return c, fmt.Errorf("invalid COOKIE_SECURE")
	}
	if !c.CookieSecure && u.Scheme != "http" {
		return c, fmt.Errorf("HTTPS requires COOKIE_SECURE")
	}
	for _, v := range []struct {
		name, def string
		target    *int
	}{{"SOLVER_MAX_NODES", "100000", &c.SolverNodes}, {"SOLVER_MAX_COMPONENT", "100", &c.MaxComponent}, {"DB_POOL_MAX", "20", &c.PoolMax}} {
		n, e := strconv.Atoi(env(v.name, v.def))
		if e != nil || n < 1 || n > 10000000 {
			return c, fmt.Errorf("invalid %s", v.name)
		}
		*v.target = n
	}
	return c, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
