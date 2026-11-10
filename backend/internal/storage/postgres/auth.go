package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"time"
)

type Account struct {
	ID, Hash   string
	MustChange bool
}

func FindAccount(ctx context.Context, db DB, email string) (Account, error) {
	var a Account
	e := db.QueryRow(ctx, "SELECT id::text,password_hash,must_change_password FROM staff_accounts WHERE lower(email)=lower($1) AND disabled_at IS NULL", email).Scan(&a.ID, &a.Hash, &a.MustChange)
	return a, MapError(e)
}
func AccountForUpdate(ctx context.Context, db DB, id string) (Account, error) {
	var a Account
	e := db.QueryRow(ctx, "SELECT id::text,password_hash,must_change_password FROM staff_accounts WHERE id=$1 AND disabled_at IS NULL FOR UPDATE", id).Scan(&a.ID, &a.Hash, &a.MustChange)
	return a, MapError(e)
}
func ActorRoles(ctx context.Context, db DB, id string) (domain.Actor, error) {
	a := domain.Actor{AccountID: id}
	rows, e := db.Query(ctx, "SELECT role FROM account_roles WHERE account_id=$1 ORDER BY role", id)
	if e != nil {
		return a, e
	}
	defer rows.Close()
	for rows.Next() {
		var r string
		if e = rows.Scan(&r); e != nil {
			return a, e
		}
		a.Roles = append(a.Roles, r)
	}
	if e = rows.Err(); e != nil {
		return a, e
	}
	var sid *string
	e = db.QueryRow(ctx, "SELECT (SELECT id::text FROM specialists WHERE account_id=$1 AND state='active')", id).Scan(&sid)
	if sid != nil {
		a.SpecialistID = *sid
	}
	return a, e
}
func CreateSession(ctx context.Context, db DB, id string, hash []byte, expires time.Time) (string, error) {
	var sid string
	e := db.QueryRow(ctx, "INSERT INTO staff_sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3) RETURNING id::text", id, hash, expires).Scan(&sid)
	return sid, e
}
func SessionAccount(ctx context.Context, db DB, hash []byte) (domain.Actor, error) {
	var a domain.Actor
	e := db.QueryRow(ctx, `SELECT s.id::text,a.id::text,a.must_change_password,s.expires_at FROM staff_sessions s JOIN staff_accounts a ON a.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND a.disabled_at IS NULL`, hash).Scan(&a.SessionID, &a.AccountID, &a.MustChangePassword, &a.ExpiresAt)
	if e != nil {
		return a, domain.AuthRequired
	}
	r, e := ActorRoles(ctx, db, a.AccountID)
	a.Roles = r.Roles
	a.SpecialistID = r.SpecialistID
	return a, e
}
func RevokeSession(ctx context.Context, db DB, id string) error {
	return Changed(ctx, db, "UPDATE staff_sessions SET revoked_at=now() WHERE id=$1", id)
}
func PasswordChanged(ctx context.Context, db DB, id, hash string) error {
	if e := Changed(ctx, db, "UPDATE staff_accounts SET password_hash=$2,must_change_password=false WHERE id=$1", id, hash); e != nil {
		return e
	}
	_, e := db.Exec(ctx, "UPDATE staff_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", id)
	return e
}
func InsertAccount(ctx context.Context, db DB, email, hash string, roles []string, mustChange bool) (string, error) {
	var id string
	e := db.QueryRow(ctx, "INSERT INTO staff_accounts(email,password_hash,must_change_password) VALUES($1,$2,$3) RETURNING id::text", email, hash, mustChange).Scan(&id)
	if e != nil {
		return "", MapError(e)
	}
	for _, r := range roles {
		if _, e = db.Exec(ctx, "INSERT INTO account_roles(account_id,role) VALUES($1,$2)", id, r); e != nil {
			return "", e
		}
	}
	return id, nil
}
func LinkAccount(ctx context.Context, db DB, specialistID, accountID string) error {
	return Changed(ctx, db, "UPDATE specialists SET account_id=$2 WHERE id=$1 AND account_id IS NULL AND state='active'", specialistID, accountID)
}
func Bootstrap(ctx context.Context, s *Store, zone, email, hash string) error {
	return s.InTx(ctx, func(db DB) error {
		if _, e := db.Exec(ctx, "SELECT pg_advisory_xact_lock(74202026)"); e != nil {
			return e
		}
		var exists bool
		if e := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM staff_accounts)").Scan(&exists); e != nil {
			return e
		}
		if exists {
			return domain.Fail("ALREADY_BOOTSTRAPPED", 409, "Bootstrap already performed")
		}
		var id string
		if e := db.QueryRow(ctx, "INSERT INTO resources(code,display_name,time_zone) VALUES('forma-tela','FORMA TELA',$1) RETURNING id::text", zone).Scan(&id); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, "INSERT INTO schedule_state(resource_id) VALUES($1)", id); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, "INSERT INTO simple_schedule(resource_id,work_days) VALUES($1,'{}')", id); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, "INSERT INTO rooms(resource_id,name) VALUES($1,'Кабинет 1')", id); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, "INSERT INTO site_settings(id) VALUES(1)"); e != nil {
			return e
		}
		if _, e := db.Exec(ctx, `INSERT INTO site_content(id,payload) VALUES(1,'{"headline":"FORMA TELA","subtitle":"Много форм. Один покой","servicesTitle":"Услуги","about":"","heroButton":"Записаться","bookingButton":"Забронировать"}')`); e != nil {
			return e
		}
		owner, e := InsertAccount(ctx, db, email, hash, []string{"owner", "admin"}, false)
		if e != nil {
			return e
		}
		return Audit(ctx, db, owner, "studio.bootstrapped", "resource", id, nil)
	})
}
func FindAccountByID(ctx context.Context, db DB, id string) (Account, error) {
	var a Account
	e := db.QueryRow(ctx, "SELECT id::text,password_hash,must_change_password FROM staff_accounts WHERE id=$1 AND disabled_at IS NULL", id).Scan(&a.ID, &a.Hash, &a.MustChange)
	return a, MapError(e)
}
