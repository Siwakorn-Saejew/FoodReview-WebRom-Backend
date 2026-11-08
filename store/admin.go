package store

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5"
)

// ---------- Dashboard ----------

type TopRated struct {
    ID          int64   `json:"id"`
    Name        string  `json:"name"`
    Cuisine     string  `json:"cuisine"`
    Rating      float64 `json:"rating"`
    ReviewCount int     `json:"reviewCount"`
}

type LatestReview struct {
    ID             int64  `json:"id"`
    RestaurantName string `json:"restaurantName"`
    AuthorName     string `json:"authorName"`
    Rating         int    `json:"rating"`
    Text           string `json:"text"`
}

type Stats struct {
    Restaurants   int64          `json:"restaurants"`
    Reviews       int64          `json:"reviews"`
    Members       int64          `json:"members"`
    Admins        int64          `json:"admins"`
    WishlistItems int64          `json:"wishlistItems"`
    TopRated      []TopRated     `json:"topRated"`
    LatestReviews []LatestReview `json:"latestReviews"`
    NewestMembers []Member       `json:"newestMembers"`
}

// Stats รวมตัวเลขของทั้งเว็บในคำขอเดียว ให้หน้า Dashboard ยิงครั้งเดียวจบ
func (s *Store) Stats(ctx context.Context) (Stats, error) {
    var st Stats
    err := s.Pool.QueryRow(ctx,
        `SELECT (SELECT count(*) FROM restaurants),
                (SELECT count(*) FROM reviews),
                (SELECT count(*) FROM members),
                (SELECT count(*) FROM members WHERE role = 'admin'),
                (SELECT count(*) FROM wishlist_items)`).
        Scan(&st.Restaurants, &st.Reviews, &st.Members, &st.Admins, &st.WishlistItems)
    if err != nil {
        return st, err
    }

    // ร้านคะแนนสูงสุด 5 ร้าน (เฉพาะร้านที่มีรีวิวแล้ว)
    rows, err := s.Pool.Query(ctx,
        `SELECT r.id, r.name, `+cuisinesSQL+`, r.rating::float8, r.review_count
         FROM restaurants r
         WHERE r.review_count > 0
         ORDER BY r.rating DESC, r.review_count DESC, r.id
         LIMIT 5`)
    if err != nil {
        return st, err
    }
    st.TopRated = []TopRated{}
    for rows.Next() {
        var t TopRated
        var cuisines []string
        if err := rows.Scan(&t.ID, &t.Name, &cuisines, &t.Rating, &t.ReviewCount); err != nil {
            rows.Close()
            return st, err
        }
        t.Cuisine = primaryCuisine(cuisines)
        st.TopRated = append(st.TopRated, t)
    }
    rows.Close()
    if err := rows.Err(); err != nil {
        return st, err
    }

    rows, err = s.Pool.Query(ctx,
        `SELECT v.id, r.name, v.author_name, v.rating, v.text
         FROM reviews v JOIN restaurants r ON r.id = v.restaurant_id
         ORDER BY v.created_at DESC, v.id DESC
         LIMIT 5`)
    if err != nil {
        return st, err
    }
    st.LatestReviews = []LatestReview{}
    for rows.Next() {
        var l LatestReview
        if err := rows.Scan(&l.ID, &l.RestaurantName, &l.AuthorName, &l.Rating, &l.Text); err != nil {
            rows.Close()
            return st, err
        }
        st.LatestReviews = append(st.LatestReviews, l)
    }
    rows.Close()
    if err := rows.Err(); err != nil {
        return st, err
    }

    rows, err = s.Pool.Query(ctx,
        `SELECT id, email, display_name, role, created_at
         FROM members ORDER BY created_at DESC, id DESC LIMIT 5`)
    if err != nil {
        return st, err
    }
    defer rows.Close()
    st.NewestMembers = []Member{}
    for rows.Next() {
        var m Member
        if err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
            return st, err
        }
        st.NewestMembers = append(st.NewestMembers, m)
    }
    return st, rows.Err()
}

// ---------- ร้านอาหาร (หลังบ้าน) ----------

// RestaurantInput คือข้อมูลที่ฟอร์มหลังบ้านส่งมา (POST / PUT /api/admin/restaurants)
// ประเภทอาหารส่งเป็น cuisines (หลายประเภท ตัวแรกคือประเภทหลัก) หรือส่ง cuisine ตัวเดียวแบบเดิมก็ได้
type RestaurantInput struct {
    Name           string   `json:"name"`
    NameTh         *string  `json:"nameTh"`
    Cuisine        string   `json:"cuisine"`
    Cuisines       []string `json:"cuisines"`
    Province       string   `json:"province"`
    District       *string  `json:"district"`
    Description    *string  `json:"description"`
    Address        *string  `json:"address"`
    Phone          *string  `json:"phone"`
    OpeningHours   *string  `json:"openingHours"`
    Menu           *string  `json:"menu"`
    Image          *string  `json:"image"`
    ImageCredit    *string  `json:"imageCredit"`
    GoogleMapsLink *string  `json:"googleMapsLink"`
    Facebook       *string  `json:"facebook"`
}

// Impact บอกว่าลบร้านแล้วอะไรจะหายตามบ้าง
type Impact struct {
    Reviews  int64 `json:"reviews"`
    Wishlist int64 `json:"wishlist"`
}

type AdminRestaurant struct {
    RestaurantDetail
    Impact Impact `json:"impact"`
}

func (s *Store) impactOf(ctx context.Context, id int64) (Impact, error) {
    var im Impact
    err := s.Pool.QueryRow(ctx,
        `SELECT (SELECT count(*) FROM reviews        WHERE restaurant_id = $1),
                (SELECT count(*) FROM wishlist_items WHERE restaurant_id = $1)`, id).Scan(&im.Reviews, &im.Wishlist)
    return im, err
}

// AdminGetRestaurant ร้านสำหรับฟอร์มแก้ไข พร้อม impact
func (s *Store) AdminGetRestaurant(ctx context.Context, id int64) (AdminRestaurant, error) {
    d, err := s.GetRestaurant(ctx, id, 0)
    if err != nil {
        return AdminRestaurant{}, err
    }
    im, err := s.impactOf(ctx, id)
    return AdminRestaurant{RestaurantDetail: d, Impact: im}, err
}

// resolveRefs แปลงชื่อจังหวัด/ประเภทเป็น id
// จังหวัดต้องมีอยู่ในตาราง provinces (ไม่มีตอบ ErrBadProvince) ส่วนประเภทใหม่ที่ยังไม่มีจะถูกเพิ่มให้
func resolveRefs(ctx context.Context, tx pgx.Tx, in RestaurantInput) (provinceID int, cuisineIDs []int, err error) {
    err = tx.QueryRow(ctx, `SELECT id FROM provinces WHERE name_th = $1`, in.Province).Scan(&provinceID)
    if errors.Is(err, pgx.ErrNoRows) {
        return 0, nil, ErrBadProvince
    }
    if err != nil {
        return 0, nil, err
    }
    for _, name := range in.Cuisines {
        var id int
        err = tx.QueryRow(ctx,
            `INSERT INTO cuisines (name) VALUES ($1)
             ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
             RETURNING id`, name).Scan(&id)
        if err != nil {
            return 0, nil, err
        }
        cuisineIDs = append(cuisineIDs, id)
    }
    return provinceID, cuisineIDs, nil
}

// saveCuisines แทนที่ประเภทอาหารทั้งหมดของร้าน (ลำดับแรก = ประเภทหลัก)
func saveCuisines(ctx context.Context, tx pgx.Tx, restaurantID int64, cuisineIDs []int) error {
    if _, err := tx.Exec(ctx, `DELETE FROM restaurant_cuisines WHERE restaurant_id = $1`, restaurantID); err != nil {
        return err
    }
    for i, id := range cuisineIDs {
        if _, err := tx.Exec(ctx,
            `INSERT INTO restaurant_cuisines (restaurant_id, cuisine_id, position) VALUES ($1, $2, $3)`,
            restaurantID, id, i); err != nil {
            return err
        }
    }
    return nil
}

func (s *Store) CreateRestaurant(ctx context.Context, in RestaurantInput) (AdminRestaurant, error) {
    tx, err := s.Pool.Begin(ctx)
    if err != nil {
        return AdminRestaurant{}, err
    }
    defer tx.Rollback(ctx) // ถ้า Commit สำเร็จแล้ว Rollback จะไม่ทำอะไร

    provinceID, cuisineIDs, err := resolveRefs(ctx, tx, in)
    if err != nil {
        return AdminRestaurant{}, err
    }
    var id int64
    err = tx.QueryRow(ctx,
        `INSERT INTO restaurants (name, name_th, province_id, district, address, description, phone,
                                  opening_hours, menu, image_url, image_credit, google_maps_link, facebook_url)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
         RETURNING id`,
        in.Name, in.NameTh, provinceID, in.District, in.Address, in.Description, in.Phone,
        in.OpeningHours, in.Menu, in.Image, in.ImageCredit, in.GoogleMapsLink, in.Facebook).Scan(&id)
    if err != nil {
        return AdminRestaurant{}, err
    }
    if err := saveCuisines(ctx, tx, id, cuisineIDs); err != nil {
        return AdminRestaurant{}, err
    }
    if err := tx.Commit(ctx); err != nil {
        return AdminRestaurant{}, err
    }
    return s.AdminGetRestaurant(ctx, id)
}

func (s *Store) UpdateRestaurant(ctx context.Context, id int64, in RestaurantInput) (AdminRestaurant, error) {
    tx, err := s.Pool.Begin(ctx)
    if err != nil {
        return AdminRestaurant{}, err
    }
    defer tx.Rollback(ctx)

    provinceID, cuisineIDs, err := resolveRefs(ctx, tx, in)
    if err != nil {
        return AdminRestaurant{}, err
    }
    tag, err := tx.Exec(ctx,
        `UPDATE restaurants
         SET name = $2, name_th = $3, province_id = $4, district = $5, address = $6, description = $7,
             phone = $8, opening_hours = $9, menu = $10, image_url = $11, image_credit = $12,
             google_maps_link = $13, facebook_url = $14, updated_at = now()
         WHERE id = $1`,
        id, in.Name, in.NameTh, provinceID, in.District, in.Address, in.Description,
        in.Phone, in.OpeningHours, in.Menu, in.Image, in.ImageCredit, in.GoogleMapsLink, in.Facebook)
    if err != nil {
        return AdminRestaurant{}, err
    }
    if tag.RowsAffected() == 0 {
        return AdminRestaurant{}, ErrNotFound
    }
    if err := saveCuisines(ctx, tx, id, cuisineIDs); err != nil {
        return AdminRestaurant{}, err
    }
    if err := tx.Commit(ctx); err != nil {
        return AdminRestaurant{}, err
    }
    return s.AdminGetRestaurant(ctx, id)
}

// DeleteRestaurant ลบร้าน รีวิว เมนู และรายการอยากไปกินของร้านนี้ถูกลบตาม (ON DELETE CASCADE ใน schema)
func (s *Store) DeleteRestaurant(ctx context.Context, id int64) error {
    tag, err := s.Pool.Exec(ctx, `DELETE FROM restaurants WHERE id = $1`, id)
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
