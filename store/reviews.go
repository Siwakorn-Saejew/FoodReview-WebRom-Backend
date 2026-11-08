package store

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

// PhotoRef คืออ้างอิงรูปของรีวิว: หน้าเว็บเอา url ไปใส่ <img src> ได้เลย (ดึงไฟล์จริงจาก GET /api/photos/{id})
type PhotoRef struct {
    ID  int64  `json:"id"`
    URL string `json:"url"`
}

func photoRef(id int64) PhotoRef {
    return PhotoRef{ID: id, URL: fmt.Sprintf("/api/photos/%d", id)}
}

// PhotoInput คือรูปที่ผ่านการตรวจชนิดและขนาดแล้ว พร้อมบันทึก
type PhotoInput struct {
    ContentType string // image/jpeg | image/png | image/webp
    Data        []byte
}

// Review คือรีวิวตามสัญญา GET /api/restaurants/{id}/reviews
type Review struct {
    ID         int64      `json:"id"`
    Rating     int        `json:"rating"`
    Text       string     `json:"text"`
    AuthorName string     `json:"authorName"`
    CreatedAt  time.Time  `json:"createdAt"`
    Photos     []PhotoRef `json:"photos"`
}

// photoRefs หารูปของรีวิวหลายอันในคำสั่งเดียว (ไม่ดึงตัวไฟล์ ดึงแค่เลข id) คืน map: เลขรีวิว -> รูป
func (s *Store) photoRefs(ctx context.Context, reviewIDs []int64) (map[int64][]PhotoRef, error) {
    out := map[int64][]PhotoRef{}
    if len(reviewIDs) == 0 {
        return out, nil
    }
    rows, err := s.Pool.Query(ctx,
        `SELECT review_id, id FROM review_photos WHERE review_id = ANY($1) ORDER BY review_id, position`, reviewIDs)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    for rows.Next() {
        var reviewID, photoID int64
        if err := rows.Scan(&reviewID, &photoID); err != nil {
            return nil, err
        }
        out[reviewID] = append(out[reviewID], photoRef(photoID))
    }
    return out, rows.Err()
}

func (s *Store) ListReviews(ctx context.Context, restaurantID int64, page, limit int) ([]Review, int, error) {
    var total int
    if err := s.Pool.QueryRow(ctx,
        `SELECT count(*) FROM reviews WHERE restaurant_id = $1`, restaurantID).Scan(&total); err != nil {
        return nil, 0, err
    }
    rows, err := s.Pool.Query(ctx,
        `SELECT id, rating, text, author_name, created_at
         FROM reviews WHERE restaurant_id = $1
         ORDER BY created_at DESC, id DESC
         LIMIT $2 OFFSET $3`, restaurantID, limit, (page-1)*limit)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    out := []Review{}
    ids := []int64{}
    for rows.Next() {
        var r Review
        if err := rows.Scan(&r.ID, &r.Rating, &r.Text, &r.AuthorName, &r.CreatedAt); err != nil {
            return nil, 0, err
        }
        out = append(out, r)
        ids = append(ids, r.ID)
    }
    if err := rows.Err(); err != nil {
        return nil, 0, err
    }
    rows.Close() // ปิดก่อนยิง query ถัดไปบน pool

    photos, err := s.photoRefs(ctx, ids)
    if err != nil {
        return nil, 0, err
    }
    for i := range out {
        out[i].Photos = photos[out[i].ID]
        if out[i].Photos == nil {
            out[i].Photos = []PhotoRef{}
        }
    }
    return out, total, nil
}

// CreateReview บันทึกรีวิวพร้อมรูปในทรานแซกชันเดียว (รีวิวไม่ผ่าน = ไม่มีรูปค้าง รูปไม่ผ่าน = ไม่มีรีวิวค้าง)
// trigger ใน DB อัปเดต rating และ review_count ของร้านให้เอง
// รีวิวซ้ำ (สมาชิกคนเดิม ร้านเดิม) ตอบ ErrReviewExists, ไม่มีร้านตอบ ErrNotFound
func (s *Store) CreateReview(ctx context.Context, restaurantID, memberID int64, rating int, text, authorName string, photos []PhotoInput) (Review, error) {
    tx, err := s.Pool.Begin(ctx)
    if err != nil {
        return Review{}, err
    }
    defer tx.Rollback(ctx) // ถ้า Commit สำเร็จแล้ว Rollback จะไม่ทำอะไร

    var r Review
    err = tx.QueryRow(ctx,
        `INSERT INTO reviews (restaurant_id, member_id, rating, text, author_name)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id, rating, text, author_name, created_at`,
        restaurantID, memberID, rating, text, authorName).
        Scan(&r.ID, &r.Rating, &r.Text, &r.AuthorName, &r.CreatedAt)
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505":
            return Review{}, ErrReviewExists
        case "23503":
            return Review{}, ErrNotFound
        }
    }
    if err != nil {
        return Review{}, err
    }

    r.Photos = []PhotoRef{}
    for i, p := range photos {
        var photoID int64
        err := tx.QueryRow(ctx,
            `INSERT INTO review_photos (review_id, position, content_type, data, size_bytes)
             VALUES ($1, $2, $3, $4, $5)
             RETURNING id`, r.ID, i, p.ContentType, p.Data, len(p.Data)).Scan(&photoID)
        if err != nil {
            return Review{}, err
        }
        r.Photos = append(r.Photos, photoRef(photoID))
    }
    if err := tx.Commit(ctx); err != nil {
        return Review{}, err
    }
    return r, nil
}

// GetPhoto ดึงไฟล์รูปจริงมาส่งให้เบราว์เซอร์ ไม่พบตอบ ErrNotFound
func (s *Store) GetPhoto(ctx context.Context, id int64) (contentType string, data []byte, err error) {
    err = s.Pool.QueryRow(ctx,
        `SELECT content_type, data FROM review_photos WHERE id = $1`, id).Scan(&contentType, &data)
    if errors.Is(err, pgx.ErrNoRows) {
        return "", nil, ErrNotFound
    }
    return contentType, data, err
}

// AdminReview คือรีวิวพร้อมชื่อร้านและรูป สำหรับหน้าตรวจรีวิวของ admin
type AdminReview struct {
    ID             int64      `json:"id"`
    RestaurantID   int64      `json:"restaurantId"`
    RestaurantName string     `json:"restaurantName"`
    Rating         int        `json:"rating"`
    Text           string     `json:"text"`
    AuthorName     string     `json:"authorName"`
    CreatedAt      time.Time  `json:"createdAt"`
    Photos         []PhotoRef `json:"photos"`
}

func (s *Store) AdminListReviews(ctx context.Context) ([]AdminReview, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT v.id, v.restaurant_id, r.name, v.rating, v.text, v.author_name, v.created_at
         FROM reviews v JOIN restaurants r ON r.id = v.restaurant_id
         ORDER BY v.created_at DESC, v.id DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []AdminReview{}
    ids := []int64{}
    for rows.Next() {
        var a AdminReview
        if err := rows.Scan(&a.ID, &a.RestaurantID, &a.RestaurantName, &a.Rating, &a.Text, &a.AuthorName, &a.CreatedAt); err != nil {
            return nil, err
        }
        out = append(out, a)
        ids = append(ids, a.ID)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    rows.Close()

    photos, err := s.photoRefs(ctx, ids)
    if err != nil {
        return nil, err
    }
    for i := range out {
        out[i].Photos = photos[out[i].ID]
        if out[i].Photos == nil {
            out[i].Photos = []PhotoRef{}
        }
    }
    return out, nil
}

// AdminDeleteReview ลบรีวิว (รูปของรีวิวถูกลบตามด้วย ON DELETE CASCADE, trigger คำนวณ rating ของร้านใหม่ให้เอง)
func (s *Store) AdminDeleteReview(ctx context.Context, id int64) error {
    tag, err := s.Pool.Exec(ctx, `DELETE FROM reviews WHERE id = $1`, id)
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
