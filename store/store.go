package store

import (
    "errors"

    "github.com/jackc/pgx/v5/pgxpool"
)

// error กลางที่ handler แปลงเป็น status code
var (
    ErrNotFound     = errors.New("not found")
    ErrEmailTaken   = errors.New("email taken")
    ErrReviewExists = errors.New("review exists")
    ErrBadProvince  = errors.New("bad province")
)

type Store struct {
    Pool *pgxpool.Pool
}
