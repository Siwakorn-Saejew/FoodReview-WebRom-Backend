# API ที่หน้าเว็บ foodjubb.com ต้องใช้ (ส่งให้ทีม Go)

ทีม: ______________________  หน้าเว็บ: foodjubb.com (Guest / Member / Admin)

กติกา: ทุก endpoint ตอบเป็น JSON (ยกเว้น `GET /api/photos/{id}` ที่ตอบไฟล์รูป), ใส่ prefix `/api`, ใช้ชื่อ field แบบ camelCase
โครงตารางฐานข้อมูลอยู่ที่ [db-schema.md](db-schema.md)

## ผู้ใช้ 3 แบบ

- **Guest** (ไม่ login): ดูร้าน ค้นหา กรอง และอ่านรีวิว (รวมรูป) ได้ ไม่ต้องมี token
- **Member** (login แล้ว): เขียนรีวิว (แนบรูปได้) เพิ่ม/ลบ/ดูร้านที่อยากไปกิน ต้องแนบ header `Authorization: Bearer <token>`
- **Admin** (member ที่ `role = "admin"`): ใช้ `/api/admin/*` ถ้าเป็น member ตอบ 403 `{ "error": "เฉพาะ admin เท่านั้น" }` ถ้าไม่มี token ตอบ 401 `{ "error": "กรุณาเข้าสู่ระบบก่อน" }`

## รายการ endpoint

| # | Method | Path | สิทธิ์ | ใช้ทำอะไร |
|---|--------|------|--------|-----------|
| 1 | GET | /api/restaurants | Guest | รายการร้าน หน้า Home และ Restaurants (ค้นหา กรอง เรียง แบ่งหน้า) |
| 2 | GET | /api/restaurants/{id} | Guest | รายละเอียดร้าน หน้า RestaurantDetail |
| 3 | GET | /api/restaurants/{id}/reviews | Guest | รีวิวของร้าน (มีรูป) |
| 4 | POST | /api/restaurants/{id}/reviews | Member | เขียนรีวิว (JSON หรือ multipart แนบรูป) |
| 5 | GET | /api/photos/{id} | Guest | ไฟล์รูปของรีวิว ใช้เป็น src ของ `<img>` |
| 6 | GET | /api/wishlist | Member | หน้า "ร้านที่อยากไปกิน" |
| 7 | POST | /api/wishlist | Member | กดหัวใจ เพิ่มร้านที่อยากไปกิน |
| 8 | DELETE | /api/wishlist/{restaurantId} | Member | กดหัวใจอีกครั้ง เอาร้านออกจากรายการ |
| 9 | POST | /api/auth/register | Guest | สมัครสมาชิก |
| 10 | POST | /api/auth/login | Guest | เข้าสู่ระบบ |
| 11 | GET | /api/me | Member | ข้อมูลสมาชิกที่ถือ token |
| 12 | GET | /api/admin/stats | Admin | ตัวเลขภาพรวม Dashboard |
| 13 | GET | /api/admin/members | Admin | รายชื่อสมาชิก |
| 14 | PUT | /api/admin/members/{id}/role | Admin | เลื่อน/ลดบทบาท |
| 15 | GET | /api/admin/restaurants/{id} | Admin | ร้านสำหรับฟอร์มแก้ไข + `impact` |
| 16 | POST | /api/admin/restaurants | Admin | เพิ่มร้าน |
| 17 | PUT | /api/admin/restaurants/{id} | Admin | แก้ไขร้าน |
| 18 | DELETE | /api/admin/restaurants/{id} | Admin | ลบร้าน (รีวิว รูป และ wishlist ของร้านถูกลบตาม) |
| 19 | GET | /api/admin/reviews | Admin | รีวิวทุกร้าน (มีรูป) |
| 20 | DELETE | /api/admin/reviews/{id} | Admin | ลบรีวิว (รูปถูกลบตาม) แล้วคำนวณ rating และ reviewCount ของร้านใหม่ |

## #1 GET /api/restaurants

Query parameter ใช้ร่วมกันได้ทั้งหมด เช่น `/api/restaurants?q=ข้าวซอย&province=เชียงใหม่&cuisine=ไทย&sort=rating`

- `q` ค้นหาจากชื่อร้าน (ไทย/อังกฤษ) หรือเมนูแนะนำ
- `province` ชื่อจังหวัดแบบเต็ม เช่น `กรุงเทพมหานคร`, `เชียงใหม่` (ไม่ส่ง = ทุกจังหวัด)
- `cuisine` ประเภทอาหาร เช่น ไทย, กาแฟ, ของหวาน ร้านที่มี **ประเภทใดประเภทหนึ่ง** ตรงกันก็ผ่าน (ร้านหนึ่งมีได้หลายประเภท)
- `sort` ค่าที่รับ: `rating` (คะแนนสูงสุดก่อน), `newest` (ใหม่สุดก่อน) ไม่ส่ง = เรียงตาม id
- `page` และ `limit` สำหรับแบ่งหน้า (ค่าเริ่มต้น page=1, limit=20, limit สูงสุด 100)

ถ้าแนบ `Authorization` มาด้วย ให้ตอบ `isWishlisted` ของสมาชิกคนนั้น ถ้าไม่แนบตอบ `false` (หน้าเว็บใช้แสดงหัวใจบนการ์ด)
ไม่มีระดับราคา (ตัดออกแล้ว)

`cuisines` คือประเภททั้งหมดของร้าน (ตัวแรกคือประเภทหลัก) ส่วน `cuisine` คือประเภทหลักตัวเดียว (= `cuisines[0]`)

```json
{
  "results": [
    {
      "id": 1,
      "name": "Baan Ayutthaya Rom",
      "nameTh": "บ้านอยุธยารมย์",
      "cuisine": "ไทย",
      "cuisines": ["ไทย"],
      "province": "พระนครศรีอยุธยา",
      "district": "พระนครศรีอยุธยา",
      "address": "1/1 หมู่ 7 ตำบลบ้านป้อม อำเภอพระนครศรีอยุธยา จังหวัดพระนครศรีอยุธยา 13000",
      "rating": 4.5,
      "reviewCount": 12,
      "image": "https://example.com/images/baan-ayutthaya-rom.jpg",
      "isWishlisted": false
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 1
}
```

## #2 GET /api/restaurants/{id}

ใช้ field ชุดเดียวกับรายการ เพิ่มดังนี้ ถ้าไม่พบ id ตอบ 404 `{ "error": "ไม่พบร้านอาหาร" }`
ถ้าแนบ `Authorization` ตอบ `isWishlisted` ของสมาชิกคนนั้น field ที่ไม่มีข้อมูลตอบ `null`

| field | ชนิด | หมายเหตุ |
|---|---|---|
| `description`, `phone`, `openingHours` | string / null | |
| `menu` | string / null | เมนูแนะนำเป็นข้อความอิสระ (ไม่ใช่ array) |
| `images` | array ของ URL | ตอนนี้มีรูปเดียว หรือ array ว่าง |
| `imageCredit` | string / null | ที่มาของรูป |
| `googleMapsLink`, `facebook` | string / null | ลิงก์ (ขึ้นต้นด้วย http/https) |
| `awards` | array | `[{ "year": 2025, "award": "Bib Gourmand" }]` ใหม่สุดก่อน หรือ array ว่าง |

```json
{
  "id": 1,
  "name": "Baan Ayutthaya Rom",
  "nameTh": "บ้านอยุธยารมย์",
  "cuisine": "ไทย",
  "cuisines": ["ไทย"],
  "province": "พระนครศรีอยุธยา",
  "district": "พระนครศรีอยุธยา",
  "address": "1/1 หมู่ 7 ตำบลบ้านป้อม อำเภอพระนครศรีอยุธยา จังหวัดพระนครศรีอยุธยา 13000",
  "description": "บรรยากาศดีๆริมแม่น้ำของเมืองโบราณ",
  "phone": "092 935 4264",
  "openingHours": null,
  "menu": "อาหารไทย กุ้งเผา",
  "rating": 4.5,
  "reviewCount": 12,
  "images": ["https://example.com/images/baan-ayutthaya-rom.jpg"],
  "imageCredit": "ที่มา: เพจ Facebook ของร้าน",
  "googleMapsLink": "https://www.google.com/maps/search/?api=1&query=14.35,100.54",
  "facebook": "https://www.facebook.com/baanayutthayarom",
  "awards": [{ "year": 2025, "award": "Bib Gourmand" }],
  "isWishlisted": true
}
```

## #3 GET /api/restaurants/{id}/reviews

รับ `page`, `limit` เหมือน #1 เรียงใหม่สุดก่อน `photos` คือรูปที่แนบกับรีวิว (ไม่มีรูปตอบ array ว่าง) `url` เอาไปต่อกับที่อยู่ backend แล้วใส่ `<img src>` ได้เลย

```json
{
  "results": [
    {
      "id": 1,
      "rating": 5,
      "text": "น้ำซุปเข้มข้นมาก เส้นเหนียวนุ่ม คุ้มราคา",
      "authorName": "สมชาย",
      "createdAt": "2026-11-01T12:30:00Z",
      "photos": [{ "id": 12, "url": "/api/photos/12" }]
    }
  ],
  "page": 1, "limit": 20, "total": 1
}
```

## #4 POST /api/restaurants/{id}/reviews (Member)

รับได้ 2 รูปแบบ:

1. **JSON** (ไม่แนบรูป) body: `{ "rating": 5, "text": "...", "authorName": "สมชาย" }` header `Content-Type: application/json`
2. **multipart/form-data** (แนบรูป) ฟิลด์ `rating`, `text`, `authorName` เป็นข้อความ และฟิลด์ `photos` เป็นไฟล์ (ส่งซ้ำชื่อเดิมได้สูงสุด 3 ไฟล์) หน้าเว็บไม่ตั้ง `Content-Type` เอง ให้เบราว์เซอร์ใส่ boundary

หน้าเว็บส่ง `authorName` จาก displayName ของสมาชิก
ตอบ 201 พร้อม `{ "id": 1, "rating": 5, "text": "...", "authorName": "สมชาย", "createdAt": "2026-11-07T10:00:00Z", "photos": [{ "id": 12, "url": "/api/photos/12" }] }`

กฎตรวจข้อมูล (ตอบ 400 พร้อม `{ "error": "ข้อความ" }` ตรวจตามลำดับนี้):

- text สั้นกว่า 10 ตัวอักษร: `{ "error": "รีวิวสั้นเกินไป" }`
- rating ไม่ใช่จำนวนเต็ม 1-5: `{ "error": "คะแนนต้องอยู่ระหว่าง 1 ถึง 5" }`
- authorName ว่าง: `{ "error": "กรุณาใส่ชื่อ" }`
- แนบรูปเกิน 3 รูป: `{ "error": "แนบรูปได้ไม่เกิน 3 รูป" }`
- รูปใหญ่เกิน 5 MB ต่อรูป: `{ "error": "รูปใหญ่เกินไป (ไม่เกิน 5 MB ต่อรูป)" }`
- ไฟล์ไม่ใช่ JPG / PNG / WebP (ตรวจจากเนื้อไฟล์ ไม่เชื่อนามสกุล): `{ "error": "ไฟล์รูปต้องเป็น JPG, PNG หรือ WebP" }`
- ทั้งคำขอใหญ่เกิน 16 MB หรืออ่าน multipart ไม่ได้: `{ "error": "ส่งรูปไม่สำเร็จ หรือไฟล์ใหญ่เกินไป (รวมไม่เกิน 16 MB)" }`
- ไม่พบร้าน: 404 `{ "error": "ไม่พบร้านอาหาร" }`
- สมาชิกคนนี้รีวิวร้านนี้ไปแล้ว: 409 `{ "error": "คุณรีวิวร้านนี้ไปแล้ว" }`

บันทึกรีวิวและรูปในทรานแซกชันเดียวกัน (รูปไม่ผ่าน = ไม่มีรีวิวค้าง)
หลังบันทึกแล้ว backend ต้องอัปเดต `rating` (ค่าเฉลี่ย ปัด 1 ตำแหน่ง) และ `reviewCount` ของร้านด้วย (ใน DB ทำด้วย trigger ดู db-schema.md)

หน้าเว็บย่อรูปก่อนส่ง (ด้านยาวไม่เกิน 1280 px แปลงเป็น JPEG) ปกติจึงเหลือหลักร้อย KB ต่อรูป

## #5 GET /api/photos/{id}

ตอบไฟล์รูป (ไม่ใช่ JSON) พร้อม `Content-Type` ตามชนิดรูป (`image/jpeg`, `image/png` หรือ `image/webp`), `X-Content-Type-Options: nosniff` และ `Cache-Control: public, max-age=31536000, immutable` ไม่ต้องมี token (ใช้เป็น `src` ของ `<img>`) ไม่พบตอบ 404 `{ "error": "ไม่พบรูป" }`

## #6-#8 ร้านที่อยากไปกิน (Member ผูกกับบัญชีด้วย token)

**GET /api/wishlist** เรียงเพิ่มล่าสุดก่อน

```json
{
  "results": [
    {
      "restaurantId": 1,
      "name": "Baan Ayutthaya Rom",
      "nameTh": "บ้านอยุธยารมย์",
      "cuisine": "ไทย",
      "cuisines": ["ไทย"],
      "province": "พระนครศรีอยุธยา",
      "district": "พระนครศรีอยุธยา",
      "address": "1/1 หมู่ 7 ตำบลบ้านป้อม ...",
      "rating": 4.5,
      "reviewCount": 12,
      "image": "https://example.com/images/baan-ayutthaya-rom.jpg",
      "addedAt": "2026-11-07T10:00:00Z"
    }
  ],
  "total": 1
}
```

**POST /api/wishlist** body `{ "restaurantId": 1 }` ตอบ 201 พร้อม `{ "restaurantId": 1, "addedAt": "..." }`

- ไม่พบร้าน: 404 `{ "error": "ไม่พบร้านอาหาร" }`
- เพิ่มร้านนี้ไปแล้ว: ตอบ 200 พร้อมข้อมูลเดิม (ไม่ error เพื่อให้กดซ้ำได้ไม่พัง)

**DELETE /api/wishlist/{restaurantId}** ตอบ 204 ไม่มี body ถ้าร้านนั้นไม่ได้อยู่ในรายการก็ตอบ 204 เหมือนกัน

ถ้าไม่มี token ทั้ง 3 ตัวตอบ 401

## #9-#11 สมาชิก

- `POST /api/auth/register` body `{ email, password, displayName }` ตอบ 201 พร้อม member / 400 กรอกไม่ครบ (รหัสผ่านอย่างน้อย 4 ตัว) / 409 อีเมลซ้ำ
- `POST /api/auth/login` body `{ email, password }` ตอบ `{ "token": "...", "member": { "id", "email", "displayName", "role", "createdAt" } }` / 401 `{ "error": "อีเมลหรือรหัสผ่านไม่ถูกต้อง" }` (ข้อความเดียวกันทั้งอีเมลผิดและรหัสผิด)
- `GET /api/me` ตอบ member ของ token นี้

## #12-#20 หลังบ้าน (Admin)

- `GET /api/admin/stats` ตอบ `{ restaurants, reviews, members, admins, wishlistItems, topRated: [{ id, name, cuisine, rating, reviewCount }], latestReviews: [{ id, restaurantName, authorName, rating, text }], newestMembers: [member] }` (topRated และ latestReviews สูงสุด 5 รายการ)
- `GET /api/admin/members` ตอบ `{ "results": [member] }`
- `PUT /api/admin/members/{id}/role` body `{ "role": "admin" | "member" }` ตอบ member ที่อัปเดตแล้ว เปลี่ยนบทบาทตัวเองไม่ได้ (400)
- `GET /api/admin/restaurants/{id}` ตอบรายละเอียดร้านแบบ #2 เพิ่ม `impact: { reviews, wishlist }`
- `POST` / `PUT /api/admin/restaurants[/{id}]` body:

  ```json
  {
    "name": "...", "nameTh": "...", "cuisines": ["ไทย", "ของหวาน"], "province": "ภูเก็ต",
    "district": "...", "address": "...", "description": "...", "phone": "...", "openingHours": "...",
    "menu": "...", "image": "https://...", "imageCredit": "...",
    "googleMapsLink": "https://...", "facebook": "https://..."
  }
  ```

  - ต้องมี `name`, `province` (ชื่อจังหวัดต้องตรงกับตาราง provinces) และประเภทอาหารอย่างน้อย 1 ประเภท (`cuisines` ไม่เกิน 6 ประเภท หรือส่ง `cuisine` ตัวเดียวแบบเดิมก็ได้) ถ้าไม่ครบตอบ 400 `{ "error": "..." }`
  - `image`, `googleMapsLink`, `facebook` ต้องขึ้นต้นด้วย http:// หรือ https:// ถ้ามี
  - ประเภทที่ยังไม่มีในระบบจะถูกเพิ่มให้ ส่วนรางวัล (`awards`) แก้ไม่ได้ผ่านหลังบ้าน
  - ตอบร้านพร้อม `impact` (POST ตอบ 201)
- `DELETE /api/admin/restaurants/{id}` ตอบ 204
- `GET /api/admin/reviews` ตอบ `{ "results": [{ id, restaurantId, restaurantName, rating, text, authorName, createdAt, photos: [{ id, url }] }] }` ใหม่สุดก่อน
- `DELETE /api/admin/reviews/{id}` ตอบ 204 ไม่พบตอบ 404 `{ "error": "ไม่พบรีวิว" }`

## เรื่องที่ต้องตกลงกัน

- CORS: ตอนพัฒนา หน้าเว็บอยู่ที่ http://localhost:3000 และโดเมน Vercel ของทีม ขอให้ backend อนุญาต 2 origin นี้ พร้อม header `Authorization`, `Content-Type` และ method `PUT`, `DELETE`
- Error: ทุก error ตอบ JSON รูปแบบเดียวกัน `{ "error": "ข้อความ" }`
- id ใช้เลขที่ backend สร้างเอง (auto increment) ใช้ให้เหมือนกันทุก endpoint
- รูปร้าน (`image`): เก็บเป็น URL ภายนอก ลิงก์จาก Facebook CDN มีวันหมดอายุ หน้าเว็บจึงแสดง emoji แทนเมื่อรูปโหลดไม่ขึ้น
- รูปรีวิว: เก็บเป็นไฟล์ในฐานข้อมูล (ตาราง `review_photos`) ไม่ต้องมีที่เก็บไฟล์แยก ถ้าในอนาคตมีรูปมากให้ย้ายไปเก็บบน object storage (S3 ฯลฯ) แล้วเก็บแค่ลิงก์
- รายชื่อจังหวัด: 77 จังหวัด ชื่อเดียวกับค่าคงที่ `PROVINCES` ใน `src/data/data.js` (ใช้ `กรุงเทพมหานคร` ไม่ใช่ `กรุงเทพฯ`)
- รายชื่อประเภทอาหาร: ภาษาไทยตามตาราง `cuisines` (21 ประเภทตั้งต้น) ค่าคงที่ `CUISINES` ใน `src/data/data.js`
