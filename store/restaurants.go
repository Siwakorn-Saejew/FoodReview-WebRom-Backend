package store

import (
    "context"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5"
)

// RestaurantItem คือร้านในรายการ (GET /api/restaurants) รูปร่างตรงกับสัญญาใน docs/restaurant-api-spec.md
// ร้านหนึ่งมีได้หลายประเภท: cuisines คือทั้งหมด (ตัวแรกคือประเภทหลัก) cuisine คือประเภทหลักตัวเดียว
type RestaurantItem struct {
    ID           int64    `json:"id"`
    Name         string   `json:"name"`
    NameTh       *string  `json:"nameTh"`
    Cuisine      string   `json:"cuisine"`
    Cuisines     []string `json:"cuisines"`
    Province     string   `json:"province"`
    District     *string  `json:"district"`
    Address      *string  `json:"address"`
    Rating       float64  `json:"rating"`
    ReviewCount  int      `json:"reviewCount"`
    Image        *string  `json:"image"`
    IsWishlisted bool     `json:"isWishlisted"`
}

// Award คือรางวัลของร้าน เช่น { year: 2025, award: "Bib Gourmand" }
type Award struct {
    Year  int    `json:"year"`
    Award string `json:"award"`
}

// RestaurantDetail คือร้านพร้อมรายละเอียด (GET /api/restaurants/{id})
type RestaurantDetail struct {
    RestaurantItem
    Description    *string  `json:"description"`
    Phone          *string  `json:"phone"`
    OpeningHours   *string  `json:"openingHours"`
    Menu           *string  `json:"menu"`
    Images         []string `json:"images"`
    ImageCredit    *string  `json:"imageCredit"`
    GoogleMapsLink *string  `json:"googleMapsLink"`
    Facebook       *string  `json:"facebook"`
    Awards         []Award  `json:"awards"`
}

type SearchParams struct {
    Q        string
    Province string
    Cuisine  string
    Sort     string // "rating" | "newest" | ว่าง
    Page     int
    Limit    int
    MemberID int64 // 0 = Guest
}

const restaurantFrom = `FROM restaurants r
    JOIN provinces p ON p.id = r.province_id`

// รายชื่อประเภททั้งหมดของร้าน r เรียงตาม position (ตัวแรกคือประเภทหลัก) ใช้ร่วมกันหลาย query
const cuisinesSQL = `(SELECT coalesce(array_agg(c.name::text ORDER BY rc.position, c.id), ARRAY[]::text[])
        FROM restaurant_cuisines rc JOIN cuisines c ON c.id = rc.cuisine_id
        WHERE rc.restaurant_id = r.id)`

// $1 = คำค้น, $2 = จังหวัด, $3 = ประเภท (ค่าว่าง = ไม่กรอง) ค่าจากผู้ใช้เดินทางผ่าน $n แยกจากคำสั่งเสมอ
// คำค้นดูจากชื่อร้าน (ไทย/อังกฤษ) และเมนูแนะนำ, ประเภทตรงกับประเภทใดประเภทหนึ่งของร้านก็ผ่าน
const restaurantWhere = `WHERE ($1::text = '' OR r.name ILIKE '%' || $1 || '%' OR r.name_th ILIKE '%' || $1 || '%'
        OR r.menu ILIKE '%' || $1 || '%')
    AND ($2::text = '' OR p.name_th = $2)
    AND ($3::text = '' OR EXISTS (SELECT 1 FROM restaurant_cuisines rc JOIN cuisines c ON c.id = rc.cuisine_id
                                   WHERE rc.restaurant_id = r.id AND c.name = $3))`

// คอลัมน์ ORDER BY ต้องมาจากรายการนี้เท่านั้น ห้ามเอาข้อความจากผู้ใช้ไปต่อ SQL ตรง ๆ
var orderBy = map[string]string{
    "rating": "r.rating DESC, r.review_count DESC, r.id",
    "newest": "r.created_at DESC, r.id DESC",
}

// escapeLike กัน % และ _ ที่ผู้ใช้พิมพ์มาถูกตีเป็นตัวแทนอักขระของ LIKE
func escapeLike(s string) string {
    r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
    return r.Replace(s)
}

// primaryCuisine คืนประเภทหลัก (ตัวแรก) หรือข้อความว่างถ้าร้านยังไม่มีประเภท
func primaryCuisine(cuisines []string) string {
    if len(cuisines) == 0 {
        return ""
    }
    return cuisines[0]
}

// SearchRestaurants ค้นหา + กรองจังหวัด/ประเภท + เรียงลำดับ + แบ่งหน้า ตอบรายการกับจำนวนทั้งหมด
func (s *Store) SearchRestaurants(ctx context.Context, p SearchParams) ([]RestaurantItem, int, error) {
    q := escapeLike(strings.TrimSpace(p.Q))

    var total int
    err := s.Pool.QueryRow(ctx,
        `SELECT count(*) `+restaurantFrom+` `+restaurantWhere,
        q, p.Province, p.Cuisine).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    order, ok := orderBy[p.Sort]
    if !ok {
        order = "r.id"
    }
    rows, err := s.Pool.Query(ctx,
        `SELECT r.id, r.name, r.name_th, `+cuisinesSQL+`, p.name_th, r.district, r.address,
                r.rating::float8, r.review_count, r.image_url,
                EXISTS (SELECT 1 FROM wishlist_items w WHERE w.member_id = $4 AND w.restaurant_id = r.id)
         `+restaurantFrom+` `+restaurantWhere+`
         ORDER BY `+order+`
         LIMIT $5 OFFSET $6`,
        q, p.Province, p.Cuisine, p.MemberID, p.Limit, (p.Page-1)*p.Limit)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    out := []RestaurantItem{}
    for rows.Next() {
        var it RestaurantItem
        if err := rows.Scan(&it.ID, &it.Name, &it.NameTh, &it.Cuisines, &it.Province, &it.District, &it.Address,
            &it.Rating, &it.ReviewCount, &it.Image, &it.IsWishlisted); err != nil {
            return nil, 0, err
        }
        it.Cuisine = primaryCuisine(it.Cuisines)
        out = append(out, it)
    }
    return out, total, rows.Err()
}

// GetRestaurant หาร้านจากเลข ไม่พบตอบ ErrNotFound memberID = 0 คือ Guest (isWishlisted = false)
func (s *Store) GetRestaurant(ctx context.Context, id, memberID int64) (RestaurantDetail, error) {
    var d RestaurantDetail
    err := s.Pool.QueryRow(ctx,
        `SELECT r.id, r.name, r.name_th, `+cuisinesSQL+`, p.name_th, r.district, r.address,
                r.rating::float8, r.review_count, r.image_url,
                EXISTS (SELECT 1 FROM wishlist_items w WHERE w.member_id = $2 AND w.restaurant_id = r.id),
                r.description, r.phone, r.opening_hours, r.menu, r.image_credit, r.google_maps_link, r.facebook_url
         `+restaurantFrom+`
         WHERE r.id = $1`, id, memberID).
        Scan(&d.ID, &d.Name, &d.NameTh, &d.Cuisines, &d.Province, &d.District, &d.Address,
            &d.Rating, &d.ReviewCount, &d.Image, &d.IsWishlisted,
            &d.Description, &d.Phone, &d.OpeningHours, &d.Menu, &d.ImageCredit, &d.GoogleMapsLink, &d.Facebook)
    if errors.Is(err, pgx.ErrNoRows) {
        return d, ErrNotFound
    }
    if err != nil {
        return d, err
    }
    d.Cuisine = primaryCuisine(d.Cuisines)

    d.Images = []string{}
    if d.Image != nil {
        d.Images = append(d.Images, *d.Image)
    }

    d.Awards = []Award{}
    rows, err := s.Pool.Query(ctx,
        `SELECT year, award FROM restaurant_awards WHERE restaurant_id = $1 ORDER BY year DESC, award`, id)
    if err != nil {
        return d, err
    }
    defer rows.Close()
    for rows.Next() {
        var a Award
        if err := rows.Scan(&a.Year, &a.Award); err != nil {
            return d, err
        }
        d.Awards = append(d.Awards, a)
    }
    return d, rows.Err()
}

// RestaurantExists ใช้เช็กก่อนอ่าน/เขียนรีวิว
func (s *Store) RestaurantExists(ctx context.Context, id int64) (bool, error) {
    var ok bool
    err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM restaurants WHERE id = $1)`, id).Scan(&ok)
    return ok, err
}
