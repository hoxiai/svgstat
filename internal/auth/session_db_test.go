package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/database"
	"github.com/joho/godotenv"
)

func TestCreateSessionStoresCurrentCreatedAt(t *testing.T) {
	_ = godotenv.Load("../../.env")
	db, err := database.New(config.Load())
	if err != nil {
		t.Skipf("Skipping test: PostgreSQL not reachable: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	m := NewManager(db.Pool, nil)
	email := fmt.Sprintf("session-test-%d@example.com", time.Now().UnixNano())
	user, err := m.Register(ctx, email, "password123", "Session Test")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer func() { _, _ = db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID) }()

	before := time.Now().Add(-time.Minute)
	session, err := m.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	after := time.Now().Add(time.Minute)

	var createdAt, expiresAt time.Time
	if err := db.Pool.QueryRow(ctx, `SELECT created_at, expires_at FROM sessions WHERE id = $1`, session.ID).Scan(&createdAt, &expiresAt); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if createdAt.Before(before) || createdAt.After(after) {
		t.Fatalf("created_at = %v, want close to now", createdAt)
	}
	if !expiresAt.After(createdAt.Add(6 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %v, want ~7 days after created_at %v", expiresAt, createdAt)
	}
	if _, err := m.ValidateSession(ctx, session.Token); err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
}
