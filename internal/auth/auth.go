package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrSessionExpired     = errors.New("session expired")
)

type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"emailVerifiedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type Manager struct {
	db          *pgxpool.Pool
	adminEmails map[string]struct{}
}

func NewManager(db *pgxpool.Pool, adminEmails []string) *Manager {
	emails := make(map[string]struct{}, len(adminEmails))
	for _, email := range adminEmails {
		emails[email] = struct{}{}
	}
	return &Manager{db: db, adminEmails: emails}
}

func (m *Manager) BootstrapAdmins(ctx context.Context) error {
	for email := range m.adminEmails {
		if _, err := m.db.Exec(ctx, `UPDATE users SET role = 'admin', updated_at = NOW() WHERE email = $1`, email); err != nil {
			return fmt.Errorf("failed to bootstrap admin %s: %w", email, err)
		}
	}
	return nil
}

func (m *Manager) Register(ctx context.Context, email, password, name string) (*User, error) {
	exists, err := m.userExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := generateID()
	now := time.Now()

	role := "user"
	if _, ok := m.adminEmails[email]; ok {
		role = "admin"
	}
	_, err = m.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, status, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $6, $6)
	`, userID, email, string(passwordHash), name, role, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:        userID,
		Email:     email,
		Name:      name,
		Status:    "active",
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (m *Manager) Login(ctx context.Context, email, password string) (*Session, error) {
	var userID string
	var passwordHash string

	err := m.db.QueryRow(ctx, `
		SELECT id, password_hash FROM users
		WHERE email = $1 AND status = 'active'
	`, email).Scan(&userID, &passwordHash)
	if err != nil {
		log.Error().Err(err).Str("email", email).Msg("user not found or inactive")
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		log.Error().Err(err).Str("email", email).Msg("invalid password")
		return nil, ErrInvalidCredentials
	}
	if _, err := m.db.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID); err != nil {
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}

	session, err := m.CreateSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

func (m *Manager) CreateSession(ctx context.Context, userID string) (*Session, error) {
	sessionID := generateID()
	token := generateToken()
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err := m.db.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $3, $4, $4)
	`, sessionID, userID, tokenHash, expiresAt)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:        sessionID,
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: expiresAt.Add(-7 * 24 * time.Hour),
	}, nil
}

func (m *Manager) ValidateSession(ctx context.Context, token string) (*User, error) {
	var user User
	var expiresAt time.Time

	err := m.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.status, u.role, u.created_at, u.updated_at, s.expires_at
		FROM users u
		JOIN sessions s ON u.id = s.user_id
		WHERE s.token_hash = $1 OR (s.token_hash IS NULL AND s.token = $2)
	`, hashToken(token), token).Scan(&user.ID, &user.Email, &user.Name, &user.Status, &user.Role, &user.CreatedAt, &user.UpdatedAt, &expiresAt)
	if err != nil {
		return nil, ErrSessionExpired
	}

	if time.Now().After(expiresAt) {
		_, _ = m.db.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1 OR (token_hash IS NULL AND token = $2)", hashToken(token), token)
		return nil, ErrSessionExpired
	}

	if user.Status != "active" {
		return nil, ErrInvalidCredentials
	}

	return &user, nil
}

func (m *Manager) Logout(ctx context.Context, token string) error {
	_, err := m.db.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1 OR (token_hash IS NULL AND token = $2)", hashToken(token), token)
	return err
}

func (m *Manager) GetUser(ctx context.Context, userID string) (*User, error) {
	var user User
	err := m.db.QueryRow(ctx, `
		SELECT id, email, name, status, role, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Name, &user.Status, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

func (m *Manager) userExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := m.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	return exists, err
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
