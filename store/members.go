package store

import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

type Member struct {
    ID           int64      `json:"id"`
    Email        string     `json:"email"`
    DisplayName  string     `json:"displayName"`
    Role         string     `json:"role"`
    CreatedAt    *time.Time `json:"createdAt"`
    PasswordHash string     `json:"-"` // ไม่มีวันออกไปกับ JSON
}

func (s *Store) CreateMember(ctx context.Context, email, hash, name string) (Member, error) {
    m := Member{Email: email, DisplayName: name}
    err := s.Pool.QueryRow(ctx,
        `INSERT INTO members (email, password_hash, display_name)
         VALUES ($1, $2, $3)
         RETURNING id, role, created_at`, email, hash, name).Scan(&m.ID, &m.Role, &m.CreatedAt)
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23505" {
        return Member{}, ErrEmailTaken
    }
    return m, err
}

func (s *Store) MemberByEmail(ctx context.Context, email string) (Member, error) {
    var m Member
    err := s.Pool.QueryRow(ctx,
        `SELECT id, email, display_name, role, created_at, password_hash
         FROM members WHERE lower(email) = $1`, email).
        Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt, &m.PasswordHash)
    if errors.Is(err, pgx.ErrNoRows) {
        return Member{}, ErrNotFound
    }
    return m, err
}

// MemberByID ใช้กับ GET /api/me: เลขมาจากตั๋วที่ตรวจแล้ว
func (s *Store) MemberByID(ctx context.Context, id int64) (Member, error) {
    var m Member
    err := s.Pool.QueryRow(ctx,
        `SELECT id, email, display_name, role, created_at
         FROM members WHERE id = $1`, id).
        Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt)
    if errors.Is(err, pgx.ErrNoRows) {
        return Member{}, ErrNotFound
    }
    return m, err
}

// ListMembers รายชื่อสมาชิกทั้งหมดสำหรับ admin เรียงคนใหม่ก่อน
func (s *Store) ListMembers(ctx context.Context) ([]Member, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT id, email, display_name, role, created_at
         FROM members ORDER BY created_at DESC, id DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []Member{}
    for rows.Next() {
        var m Member
        if err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
            return nil, err
        }
        out = append(out, m)
    }
    return out, rows.Err()
}

// UpdateMemberRole เปลี่ยนบทบาท (member <-> admin) ไม่พบคนตอบ ErrNotFound
func (s *Store) UpdateMemberRole(ctx context.Context, id int64, role string) (Member, error) {
    var m Member
    err := s.Pool.QueryRow(ctx,
        `UPDATE members SET role = $2, updated_at = now()
         WHERE id = $1
         RETURNING id, email, display_name, role, created_at`, id, role).
        Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt)
    if errors.Is(err, pgx.ErrNoRows) {
        return Member{}, ErrNotFound
    }
    return m, err
}
