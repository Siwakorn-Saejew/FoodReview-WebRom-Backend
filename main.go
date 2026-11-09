package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    _ "github.com/joho/godotenv/autoload" // โหลด .env ให้เองถ้ามีไฟล์ (ในเครื่อง) บน server จริงไม่มีไฟล์ก็เงียบ

    "foodjubb/auth"
    "foodjubb/handler"
    "foodjubb/mw"
    "foodjubb/store"
)

func loadEnv(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    for _, rawLine := range strings.Split(string(data), "\n") {
        line := strings.TrimSpace(rawLine)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        value = strings.Trim(value, `"'`)
        if key == "" {
            continue
        }
        if _, exists := os.LookupEnv(key); !exists {
            if err := os.Setenv(key, value); err != nil {
                return err
            }
        }
    }

    return nil
}

func main() {
    if err := loadEnv(".env"); err != nil {
        log.Fatal("ไม่สามารถอ่านไฟล์ .env ได้: ", err)
    }

    secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
    if len(secret) < 32 {
        log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
    }

    databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
    if databaseURL == "" {
        log.Fatal("DATABASE_URL ยังไม่ได้ตั้งค่า")
    }

    pool, err := pgxpool.New(context.Background(), databaseURL)
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()
    if err := pool.Ping(context.Background()); err != nil {
        log.Fatal("ต่อฐานข้อมูลไม่ติด ", err)
    }

    tickets := &auth.Issuer{Secret: []byte(secret), TTL: time.Hour}
    h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}

    // optional = Guest ผ่านได้ (ถ้ามีตั๋วจะตอบ isWishlisted)
    // login = ต้องมีตั๋ว (Member), admin = ต้องมีตั๋วและบทบาทเป็น admin (ตรวจตั๋วก่อนเสมอ ค่อยดูบทบาท)
    optional := func(next http.HandlerFunc) http.HandlerFunc { return mw.OptionalLogin(tickets, next) }
    login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(tickets, next) }
    admin := func(next http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole("admin", next)) }

    mux := http.NewServeMux()

    // สมาชิก
    mux.HandleFunc("POST /api/auth/register", h.Register)
    mux.HandleFunc("POST /api/auth/login", h.Login)
    mux.HandleFunc("GET /api/me", login(h.Me))

    // ร้านอาหาร + รีวิว: Guest ดูได้ เขียนรีวิวต้องเป็น Member
    mux.HandleFunc("GET /api/restaurants", optional(h.SearchRestaurants))
    mux.HandleFunc("GET /api/restaurants/{id}", optional(h.GetRestaurant))
    mux.HandleFunc("GET /api/restaurants/{id}/reviews", optional(h.ListReviews)) // แนบตั๋ว = ได้ isMine ของรีวิวตัวเอง
    mux.HandleFunc("POST /api/restaurants/{id}/reviews", login(h.CreateReview)) // JSON หรือ multipart (แนบรูปได้ 3 รูป)
    mux.HandleFunc("DELETE /api/reviews/{id}", login(h.DeleteReview))           // ลบได้เฉพาะรีวิวของตัวเอง
    mux.HandleFunc("GET /api/photos/{id}", h.GetPhoto)                         // ไฟล์รูปของรีวิว ใช้เป็น src ของ <img>

    // ร้านที่อยากไปกิน: เฉพาะ Member
    mux.HandleFunc("GET /api/wishlist", login(h.ListWishlist))
    mux.HandleFunc("POST /api/wishlist", login(h.AddWishlist))
    mux.HandleFunc("DELETE /api/wishlist/{restaurantId}", login(h.RemoveWishlist))

    // หลังบ้าน: ทุกเส้นทางใต้ /api/admin ผ่าน admin(...) ไม่มีข้อยกเว้น
    mux.HandleFunc("GET /api/admin/stats", admin(h.AdminStats))
    mux.HandleFunc("GET /api/admin/members", admin(h.AdminMembers))
    mux.HandleFunc("PUT /api/admin/members/{id}/role", admin(h.AdminSetRole))
    mux.HandleFunc("GET /api/admin/restaurants/{id}", admin(h.AdminGetRestaurant))
    mux.HandleFunc("POST /api/admin/restaurants", admin(h.AdminCreateRestaurant))
    mux.HandleFunc("PUT /api/admin/restaurants/{id}", admin(h.AdminUpdateRestaurant))
    mux.HandleFunc("DELETE /api/admin/restaurants/{id}", admin(h.AdminDeleteRestaurant))
    mux.HandleFunc("GET /api/admin/reviews", admin(h.AdminReviews))
    mux.HandleFunc("DELETE /api/admin/reviews/{id}", admin(h.AdminDeleteReview))

    // path ใต้ /api ที่ไม่มีอยู่ตอบ JSON (ไม่ใช่ข้อความธรรมดา)
    mux.HandleFunc("/api/", handler.NotFound)

    origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
    if len(strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))) == 0 {
        origins = []string{"http://localhost:3000", "https://foodjubb-restaurant-review.vercel.app"}
    }

    port := os.Getenv("PORT") // server จริงส่งมาให้ ในเครื่องไม่มีก็ใช้ 8080
    if port == "" {
        port = "8080"
    }

    log.Println("FoodJubb API ฟังอยู่ที่ :"+port, "ต้อนรับ", origins)
    log.Fatal(http.ListenAndServe(":"+port, mw.Logging(mw.CORS(origins, mux))))
}
