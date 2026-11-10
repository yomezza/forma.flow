package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"net/mail"
	"strings"
	"time"
)

type Service struct {
	Store     *postgres.Store
	TTL       time.Duration
	DummyHash string
}

func New(db *postgres.Store, ttl time.Duration) *Service {
	dummy, _ := Hash("unused-random-dummy-password")
	return &Service{Store: db, TTL: ttl, DummyHash: dummy}
}
func tokenHash(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
func Secret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func CSRF(token string) string {
	h := sha256.Sum256([]byte("ft-csrf:" + token))
	return hex.EncodeToString(h[:])
}
func Session(a domain.Actor) domain.M {
	role := "specialist"
	if a.Has("admin") {
		role = "admin"
	} else if a.Has("owner") {
		role = "owner"
	}
	return domain.M{"accountId": a.AccountID, "roles": a.Roles, "activeRole": role, "mustChangePassword": a.MustChangePassword, "expiresAt": a.ExpiresAt.UTC().Format(time.RFC3339Nano), "csrfToken": CSRF(a.Token)}
}
func (s *Service) issue(ctx context.Context, db postgres.DB, id string, must bool) (domain.Actor, error) {
	a, e := postgres.ActorRoles(ctx, db, id)
	if e != nil {
		return a, e
	}
	a.Token = Secret()
	a.ExpiresAt = time.Now().Add(s.TTL)
	a.MustChangePassword = must
	a.SessionID, e = postgres.CreateSession(ctx, db, id, tokenHash(a.Token), a.ExpiresAt)
	return a, e
}
func (s *Service) Login(ctx context.Context, email, password string) (out domain.Actor, err error) {
	a, e := postgres.FindAccount(ctx, s.Store.Pool, email)
	hash := a.Hash
	if e != nil {
		hash = s.DummyHash
	}
	ok := Verify(hash, password)
	if !ok || e != nil {
		_ = postgres.Audit(ctx, s.Store.Pool, "", "auth.login_failed", "authentication", "", domain.M{})
		return out, domain.AuthRequired
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		current, e := postgres.AccountForUpdate(ctx, db, a.ID)
		if e != nil || current.Hash != a.Hash {
			return domain.AuthRequired
		}
		out, e = s.issue(ctx, db, a.ID, current.MustChange)
		if e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.ID, "auth.login", "account", a.ID, nil)
	})
	return
}
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Actor, error) {
	if len(token) != 43 {
		return domain.Actor{}, domain.AuthRequired
	}
	a, e := postgres.SessionAccount(ctx, s.Store.Pool, tokenHash(token))
	a.Token = token
	return a, e
}
func (s *Service) Logout(ctx context.Context, a domain.Actor) error {
	return postgres.RevokeSession(ctx, s.Store.Pool, a.SessionID)
}
func (s *Service) ChangePassword(ctx context.Context, a domain.Actor, current, next string) (out domain.Actor, err error) {
	if !domain.Text(next, 12, 1024) || current == next {
		return out, domain.Validation("newPassword")
	}
	account, e := postgres.FindAccountByID(ctx, s.Store.Pool, a.AccountID)
	if e != nil || !Verify(account.Hash, current) {
		return out, domain.AuthRequired
	}
	hash, e := Hash(next)
	if e != nil {
		return out, e
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		locked, e := postgres.AccountForUpdate(ctx, db, a.AccountID)
		if e != nil || locked.Hash != account.Hash {
			return domain.AuthRequired
		}
		if e = postgres.PasswordChanged(ctx, db, a.AccountID, hash); e != nil {
			return e
		}
		out, e = s.issue(ctx, db, a.AccountID, false)
		if e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.AccountID, "auth.password_changed", "account", a.AccountID, nil)
	})
	return
}
func (s *Service) Invite(ctx context.Context, a domain.Actor, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.String("email")))
	parsed, e := mail.ParseAddress(email)
	if e != nil || parsed.Address != email || !domain.Text(email, 3, 255) {
		return nil, domain.Validation("email")
	}
	roles := in.Strings("roles")
	if len(roles) == 0 {
		return nil, domain.Validation("roles")
	}
	seen := map[string]bool{}
	for _, r := range roles {
		if (r != "admin" && r != "specialist") || seen[r] {
			return nil, domain.Validation("roles")
		}
		seen[r] = true
	}
	sid := in.String("specialistId")
	if seen["specialist"] != (sid != "") {
		return nil, domain.Validation("specialistId")
	}
	secret := Secret()
	hash, e := Hash(secret)
	if e != nil {
		return nil, e
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		id, e := postgres.InsertAccount(ctx, db, email, hash, roles, true)
		if e != nil {
			return e
		}
		if sid != "" {
			if e = postgres.LinkAccount(ctx, db, sid, id); e != nil {
				return e
			}
		}
		out = domain.M{"accountId": id, "email": email, "roles": roles, "specialistId": in["specialistId"], "mustChangePassword": true, "temporaryPassword": secret}
		return postgres.Audit(ctx, db, a.AccountID, "staff.invited", "account", id, domain.M{"roles": roles})
	})
	return
}
