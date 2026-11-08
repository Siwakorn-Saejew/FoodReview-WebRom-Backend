# foodjubb.com Backend (Go + PostgreSQL)

API ของเว็บรีวิวร้านอาหาร foodjubb.com ทำตามสัญญาใน [`docs/restaurant-api-spec.md`](docs/restaurant-api-spec.md)
และออกแบบฐานข้อมูลตาม [`docs/db-schema.md`](docs/db-schema.md) โครงโปรเจกต์เหมือน MovieHub backend

Tech stack: Go (`net/http` + `ServeMux`), PostgreSQL, `pgx/v5` (connection pool), JWT (`golang-jwt/v5`), bcrypt, `godotenv`

## เริ่มใช้งาน

Docker ใช้รันแค่ฐานข้อมูล (PostgreSQL) ส่วน Go รันด้วย `go run .` ในเครื่อง (ต้องติดตั้ง Go และ Docker)

go get github.com/golang-jwt/jwt/v5 #เตรียมjwt
openssl rand -hex 32 > .jwt_secret
echo .jwt_secret >> .gitignore
export JWT_SECRET=$(cat .jwt_secret)
echo ${#JWT_SECRET} #แล้วเอาไฟล์จาก.jwt_secret ไปใส่ใน.env

```bash
cp .env.example .env                  # แก้ JWT_SECRET ตามต้องการ (ยาวอย่างน้อย 32 ตัวอักษร)

docker compose up -d                  # 1) เปิด Postgres (ฐานข้อมูลชื่อ kitchen พอร์ต 5432)

# 2) สร้างตารางและใส่ข้อมูลทดลอง (ทำครั้งเดียว ตอนฐานข้อมูลยังว่าง)
docker compose exec -T db psql -U postgres -d kitchen < schema.sql
docker compose exec -T db psql -U postgres -d kitchen < seed.sql

go run .                              # 3) เปิด API ที่พอร์ต 4000
```

เมื่อเห็นข้อความ `FoodJubb API ฟังอยู่ที่ :4000` ลองเรียก

```bash
curl http://localhost:4000/api/restaurants
```

พอร์ต 4000 ตรงกับ `"proxy": "http://localhost:4000"` ของหน้าเว็บ React จึงไม่ต้องแก้อะไรฝั่งหน้าเว็บ แค่ปิด `npm run mock` แล้ว `npm start`

| คำสั่ง | ทำอะไร |
|---|---|
| `docker compose up -d` | เปิดฐานข้อมูล |
| `docker compose down` | ปิด (ข้อมูลยังอยู่ใน volume `pgdata`) |
| `docker compose down -v` | ปิดและลบข้อมูลทั้งหมด ต้องรัน schema.sql กับ seed.sql ใหม่ |

> ฐานข้อมูลใน compose นี้ไม่ได้สร้างตารางให้อัตโนมัติ ต้องรัน `schema.sql` เองตามข้อ 2 ถ้ารันซ้ำตอนมีตารางอยู่แล้วจะ error (ให้ `down -v` ก่อน)

ใช้ฐานข้อมูลบนคลาวด์ (เช่น Supabase) แทนได้ โดยตั้ง `DATABASE_URL` เป็นลิงก์ของเขา แล้วรัน `schema.sql` กับ `seed.sql` ตรง ๆ

## บัญชีทดลอง (รหัสผ่านทุกบัญชี = `1234`)

| อีเมล | บทบาท |
|---|---|
| `demo@foodjubb.test` | member |
| `admin@foodjubb.test` | admin |

**ห้ามเอาบัญชีและ seed นี้ขึ้น server จริง**

## ข้อมูลร้านอาหาร (จาก CSV)

`seed.sql` สร้างจาก `data/restaurants_100.csv` (ร้าน 100 ร้านใน 47 จังหวัด) ด้วยสคริปต์ `scripts/make_seed.py`
ห้ามแก้ `seed.sql` ตรง ๆ ให้แก้ CSV แล้วรันใหม่ แล้วโหลดเข้าฐานข้อมูลอีกรอบ (`docker compose down -v` ก่อน)

```bash
python3 scripts/make_seed.py
```

สคริปต์ทำให้ดังนี้
- แปลงประเภทอาหารภาษาอังกฤษเป็นภาษาไทย (ร้านหนึ่งมีได้หลายประเภท) ถ้าเจอประเภทที่ไม่รู้จักจะหยุดและบอกชื่อ ให้เพิ่มใน `CUISINE_MAP` และใน `schema.sql`
- แยกคอลัมน์ `michelin` (เช่น `2022:Bib Gourmand,2023:Bib Gourmand`) เป็นตาราง `restaurant_awards`
- เติม `https://` ให้ลิงก์ Facebook ที่ไม่มี และข้ามลิงก์รูปที่ฝังเป็น `data:image` (ยาวมาก)
- **ไม่มีรีวิวตั้งต้น** ร้านทุกร้านเริ่มที่คะแนน 0 / "ยังไม่มีรีวิว" รีวิวต้องมาจากสมาชิกจริง

**เรื่องรูปร้านใน CSV:** ลิงก์รูป 73 รูปเป็นลิงก์จาก Facebook CDN ที่มีวันหมดอายุฝังอยู่ในลิงก์ (ประมาณ 12 พ.ย. ถึง 8 ธ.ค. 2026) หลังจากนั้นรูปจะโหลดไม่ขึ้น หน้าเว็บจะแสดง emoji ของประเภทอาหารแทนโดยอัตโนมัติ
รูปเป็นของร้านแต่ละร้าน มีเครดิตที่มาเก็บไว้ในคอลัมน์ `image_credit` และแสดงใต้รูปในหน้าร้าน

## รูปในรีวิว

สมาชิกแนบรูปได้สูงสุด 3 รูปต่อรีวิว (ถ่ายจากกล้องมือถือหรือเลือกจากเครื่อง) หน้าเว็บย่อรูปให้ด้านยาวไม่เกิน 1280 px ก่อนส่ง
Backend รับแบบ `multipart/form-data` ที่ `POST /api/restaurants/{id}/reviews` ตรวจชนิดไฟล์จากเนื้อไฟล์จริง (JPG / PNG / WebP) ขนาดไม่เกิน 5 MB ต่อรูป แล้วเก็บในตาราง `review_photos` (`bytea`) พร้อมรีวิวในทรานแซกชันเดียว
ดึงรูปได้ที่ `GET /api/photos/{id}` (ไม่ต้อง login) เหตุผลที่เลือกเก็บในฐานข้อมูลอยู่ใน `docs/db-schema.md` หัวข้อ 6

ทดลองส่งรีวิวพร้อมรูปด้วย curl:

```bash
curl -s -X POST localhost:4000/api/restaurants/1/reviews -H "Authorization: Bearer $TOKEN" \
  -F rating=5 -F "text=ร้านนี้บรรยากาศดีมาก อาหารอร่อย" -F authorName=Demo \
  -F photos=@./food1.jpg -F photos=@./food2.jpg
```

## Endpoint ทั้งหมด

| Method | Path | สิทธิ์ |
|---|---|---|
| POST | `/api/auth/register` | Guest |
| POST | `/api/auth/login` | Guest |
| GET | `/api/me` | Member |
| GET | `/api/restaurants` (`q`, `province`, `cuisine`, `sort`, `page`, `limit`) | Guest (แนบ token = ได้ `isWishlisted`) |
| GET | `/api/restaurants/{id}` | Guest (แนบ token = ได้ `isWishlisted`) |
| GET | `/api/restaurants/{id}/reviews` | Guest |
| POST | `/api/restaurants/{id}/reviews` (JSON หรือ multipart แนบรูป) | Member |
| GET | `/api/photos/{id}` | Guest (ไฟล์รูปของรีวิว) |
| GET / POST | `/api/wishlist` | Member |
| DELETE | `/api/wishlist/{restaurantId}` | Member |
| GET | `/api/admin/stats`, `/api/admin/members`, `/api/admin/reviews` | Admin |
| PUT | `/api/admin/members/{id}/role` | Admin |
| GET / PUT / DELETE | `/api/admin/restaurants/{id}` | Admin |
| POST | `/api/admin/restaurants` | Admin |
| DELETE | `/api/admin/reviews/{id}` | Admin |

ทุก error ตอบ `{ "error": "ข้อความ" }` ไม่มี token = 401, ไม่ใช่ admin = 403
รายละเอียด JSON ของแต่ละตัวอยู่ใน `docs/restaurant-api-spec.md`

## ทดลองด้วย curl

```bash
# เข้าสู่ระบบ ได้ token
curl -s -X POST localhost:4000/api/auth/login -H "Content-Type: application/json" \
  -d '{"email":"demo@foodjubb.test","password":"1234"}'

TOKEN=<ใส่ token ที่ได้>

# ค้นหาร้านในเชียงใหม่ ประเภทไทย
curl -s "localhost:4000/api/restaurants?province=เชียงใหม่&cuisine=ไทย&sort=rating"

# เพิ่มร้านที่อยากไปกิน
curl -s -X POST localhost:4000/api/wishlist -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"restaurantId":2}'

# เขียนรีวิว
curl -s -X POST localhost:4000/api/restaurants/12/reviews -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"rating":5,"text":"อาหารอร่อยมาก ราคาดี","authorName":"Demo"}'

# member เรียกหลังบ้าน ต้องได้ 403
curl -i localhost:4000/api/admin/stats -H "Authorization: Bearer $TOKEN"
```

## โครงสร้างไฟล์

```
main.go              ตั้งค่า อ่าน .env ต่อฐานข้อมูล ลงทะเบียนเส้นทาง (ใครเข้าเส้นไหนได้บ้างดูที่นี่)
auth/                bcrypt (รหัสผ่าน) และ JWT (ตั๋ว อายุ 1 ชั่วโมง)
mw/                  RequireLogin / OptionalLogin / RequireRole, CORS, Logging
handler/             รับคำขอ ตรวจข้อมูล แปลง error เป็น status code
  auth.go            สมัคร / login / me
  restaurants.go     รายการและรายละเอียดร้าน
  reviews.go         รีวิว + อัปโหลดรูป + ส่งไฟล์รูป
  wishlist.go        ร้านที่อยากไปกิน
  admin.go           หลังบ้านทั้งหมด
store/               SQL ทั้งหมด (ค่าจากผู้ใช้ส่งผ่าน $1 $2 เสมอ ไม่ต่อสตริง)
schema.sql           สร้างตาราง + trigger คำนวณคะแนน + จังหวัด 77 จังหวัด + ประเภทอาหาร 21 ประเภท
seed.sql             ร้าน 100 ร้าน + บัญชีทดลอง (สร้างจาก CSV ห้ามแก้ตรง ๆ)
data/restaurants_100.csv   ข้อมูลร้านต้นฉบับ
scripts/make_seed.py       แปลง CSV เป็น seed.sql
docs/                สัญญา API และ schema (ทีมหน้าเว็บกับทีม Go ใช้ร่วมกัน)
docker-compose.yml   เปิด PostgreSQL อย่างเดียว
```

## ข้อควรรู้

- ตั๋ว (JWT) อายุ 1 ชั่วโมง หมดแล้วต้อง login ใหม่ ตัวเลขอยู่ที่ `main.go` (`TTL`)
- รหัสผ่านขั้นต่ำ 4 ตัวตามที่หน้าเว็บกำหนด (moviehub ใช้ 8) ปรับที่ `minPasswordLen` ใน `handler/auth.go`
- CORS อนุญาตเฉพาะ origin ใน `ALLOWED_ORIGINS` (ตอน deploy ใส่โดเมน Vercel ของทีม)
- สมาชิก 1 คนรีวิวร้านเดียวกันได้ 1 ครั้ง (ซ้ำตอบ 409) คะแนนเฉลี่ยของร้านคำนวณด้วย trigger ใน DB
- รันเทสต์: `go test ./...` (เทสต์ token, รหัสผ่าน, การตรวจข้อมูลร้าน/รีวิว/ชนิดไฟล์รูป และการอ่าน .env ไม่ต้องต่อฐานข้อมูล)
- ควรสำรองฐานข้อมูลสม่ำเสมอ เพราะรูปรีวิวเก็บอยู่ในนั้นด้วย
