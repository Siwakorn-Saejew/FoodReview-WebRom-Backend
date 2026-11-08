package store

import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

// WishlistItem คือร้านในรายการที่อยากไปกิน ตามสัญญา GET /api/wishlist
type WishlistItem struct {
    RestaurantID int64     `json:"restaurantId"`
    Name         string    `json:"name"`
    NameTh       *string   `json:"nameTh"`
    Cuisine      string    `json:"cuisine"`
    Cuisines     []string  `json:"cuisines"`
    Province     string    `json:"province"`
    District     *string   `json:"district"`
    Address      *string   `json:"address"`
    Rating       float64   `json:"rating"`
    ReviewCount  int       `json:"reviewCount"`
    Image        *string   `json:"image"`
    AddedAt      time.Time `json:"addedAt"`
}

// ListWishlist เรียงเพิ่มล่าสุดก่อน เห็นเฉพาะของสมาชิกคนที่ถือตั๋ว
func (s *Store) ListWishlist(ctx context.Context, memberID int64) ([]WishlistItem, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT r.id, r.name, r.name_th, `+cuisinesSQL+`, p.name_th, r.district, r.address,
                r.rating::float8, r.review_count, r.image_url, w.added_at
         FROM wishlist_items w
         JOIN restaurants r ON r.id = w.restaurant_id
         JOIN provinces   p ON p.id = r.province_id
         WHERE w.member_id = $1
         ORDER BY w.added_at DESC, r.id DESC`, memberID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []WishlistItem{}
    for rows.Next() {
        var it WishlistItem
        if err := rows.Scan(&it.RestaurantID, &it.Name, &it.NameTh, &it.Cuisines, &it.Province, &it.District,
            &it.Address, &it.Rating, &it.ReviewCount, &it.Image, &it.AddedAt); err != nil {
            return nil, err
        }
        it.Cuisine = primaryCuisine(it.Cuisines)
        out = append(out, it)
    }
    return out, rows.Err()
}

// AddWishlist เพิ่มร้าน ถ้ามีอยู่แล้วคืนวันที่เดิมและ created = false (กดซ้ำไม่ error)
// ไม่มีร้านนี้ตอบ ErrNotFound
func (s *Store) AddWishlist(ctx context.Context, memberID, restaurantID int64) (addedAt time.Time, created bool, err error) {
    err = s.Pool.QueryRow(ctx,
        `INSERT INTO wishlist_items (member_id, restaurant_id) VALUES ($1, $2)
         ON CONFLICT (member_id, restaurant_id) DO NOTHING
         RETURNING added_at`, memberID, restaurantID).Scan(&addedAt)
    if err == nil {
        return addedAt, true, nil
    }
    if errors.Is(err, pgx.ErrNoRows) { // ชน PRIMARY KEY = มีอยู่แล้ว
        err = s.Pool.QueryRow(ctx,
            `SELECT added_at FROM wishlist_items WHERE member_id = $1 AND restaurant_id = $2`,
            memberID, restaurantID).Scan(&addedAt)
        return addedAt, false, err
    }
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key: ไม่มีร้านนี้
        return time.Time{}, false, ErrNotFound
    }
    return time.Time{}, false, err
}

// RemoveWishlist ลบร้านออกจากรายการ ไม่อยู่ในรายการก็ไม่ error
func (s *Store) RemoveWishlist(ctx context.Context, memberID, restaurantID int64) error {
    _, err := s.Pool.Exec(ctx,
        `DELETE FROM wishlist_items WHERE member_id = $1 AND restaurant_id = $2`, memberID, restaurantID)
    return err
}
