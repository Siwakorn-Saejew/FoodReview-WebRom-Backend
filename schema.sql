-- foodjubb: สร้างตารางทั้งหมด (PostgreSQL 14+) รันครั้งเดียวตอนฐานข้อมูลยังว่าง
-- คำอธิบายและ ER diagram อยู่ใน docs/db-schema.md

BEGIN;

-- ---------- ข้อมูลอ้างอิง ----------
CREATE TABLE provinces (
  id        smallserial PRIMARY KEY,
  name_th   varchar(60) NOT NULL UNIQUE
);

CREATE TABLE cuisines (
  id        smallserial PRIMARY KEY,
  name      varchar(60) NOT NULL UNIQUE
);

-- ---------- สมาชิก ----------
CREATE TABLE members (
  id             bigserial PRIMARY KEY,
  email          varchar(255) NOT NULL,
  password_hash  varchar(100) NOT NULL,                       -- bcrypt ห้ามเก็บรหัสผ่านตรง ๆ
  display_name   varchar(60)  NOT NULL,
  role           varchar(10)  NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
  created_at     timestamptz  NOT NULL DEFAULT now(),
  updated_at     timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX members_email_uk ON members (lower(email));    -- อีเมลซ้ำไม่ได้ ไม่สนตัวพิมพ์

-- ---------- ร้านอาหาร ----------
CREATE TABLE restaurants (
  id                bigserial PRIMARY KEY,
  name              varchar(150) NOT NULL,
  name_th           varchar(150),
  province_id       smallint     NOT NULL REFERENCES provinces (id),
  district          varchar(80),
  address           varchar(255),
  description       text,
  phone             varchar(60),                               -- บางร้านมีหลายเบอร์ คั่นด้วย / หรือ ,
  opening_hours     varchar(100),
  menu              text,                                      -- เมนูแนะนำ เป็นข้อความอิสระ ใช้ค้นหาด้วย q
  image_url         text,                                      -- ลิงก์รูป (ยาวได้มาก จึงใช้ text)
  image_credit      text,                                      -- ที่มาของรูป
  google_maps_link  text,
  facebook_url      text,
  rating            numeric(2,1) NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),   -- ค่าเฉลี่ยจาก reviews (trigger ด้านล่างอัปเดตให้)
  review_count      integer      NOT NULL DEFAULT 0 CHECK (review_count >= 0),
  created_at        timestamptz  NOT NULL DEFAULT now(),
  updated_at        timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX restaurants_province_idx ON restaurants (province_id);       -- กรองตามจังหวัด
CREATE INDEX restaurants_rating_idx   ON restaurants (rating DESC, review_count DESC);   -- sort=rating
CREATE INDEX restaurants_created_idx  ON restaurants (created_at DESC);   -- sort=newest

-- ร้านหนึ่งมีได้หลายประเภท (เช่น ไทย + ของหวาน + กาแฟ) position 0 = ประเภทหลัก
CREATE TABLE restaurant_cuisines (
  restaurant_id  bigint   NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
  cuisine_id     smallint NOT NULL REFERENCES cuisines (id),
  position       smallint NOT NULL DEFAULT 0,
  PRIMARY KEY (restaurant_id, cuisine_id)
);
CREATE INDEX restaurant_cuisines_cuisine_idx ON restaurant_cuisines (cuisine_id);   -- กรองตามประเภท

-- รางวัล เช่น Michelin Bib Gourmand ร้านหนึ่งได้หลายปี
CREATE TABLE restaurant_awards (
  restaurant_id  bigint      NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
  year           smallint    NOT NULL,
  award          varchar(60) NOT NULL,
  PRIMARY KEY (restaurant_id, year, award)
);

-- ---------- รีวิว ----------
CREATE TABLE reviews (
  id             bigserial PRIMARY KEY,
  restaurant_id  bigint      NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
  member_id      bigint      NOT NULL REFERENCES members (id) ON DELETE CASCADE,
  rating         smallint    NOT NULL CHECK (rating BETWEEN 1 AND 5),
  text           text        NOT NULL CHECK (char_length(btrim(text)) >= 10),
  author_name    varchar(60) NOT NULL CHECK (char_length(btrim(author_name)) >= 1),   -- เก็บชื่อตอนรีวิว ตรงกับ authorName ใน API
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (member_id, restaurant_id)                                       -- รีวิวร้านเดียวกันได้คนละครั้ง (ซ้ำ = 409)
);
CREATE INDEX reviews_restaurant_idx ON reviews (restaurant_id, created_at DESC);

-- รูปที่แนบกับรีวิว (ถ่ายหรือเลือกจากเครื่อง) เก็บไฟล์ในฐานข้อมูลเลย ลบรีวิวแล้วรูปหายตาม
-- รีวิวหนึ่งแนบได้สูงสุด 3 รูป (position 0-2) รูปละไม่เกิน 5 MB เฉพาะ JPG / PNG / WebP
CREATE TABLE review_photos (
  id            bigserial PRIMARY KEY,
  review_id     bigint      NOT NULL REFERENCES reviews (id) ON DELETE CASCADE,
  position      smallint    NOT NULL CHECK (position BETWEEN 0 AND 2),
  content_type  varchar(20) NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
  data          bytea       NOT NULL,
  size_bytes    integer     NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (review_id, position)
);

-- ---------- ร้านที่อยากไปกิน ----------
CREATE TABLE wishlist_items (
  member_id      bigint      NOT NULL REFERENCES members (id) ON DELETE CASCADE,
  restaurant_id  bigint      NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
  added_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (member_id, restaurant_id)                                  -- กดซ้ำไม่เพิ่มแถว (INSERT ... ON CONFLICT DO NOTHING)
);
CREATE INDEX wishlist_member_idx ON wishlist_items (member_id, added_at DESC);

-- ---------- คำนวณ rating / review_count ของร้านใหม่ทุกครั้งที่รีวิวเพิ่ม แก้ หรือลบ ----------
CREATE FUNCTION refresh_restaurant_rating() RETURNS trigger AS $$
DECLARE rid bigint;
BEGIN
  IF TG_OP = 'DELETE' THEN rid := OLD.restaurant_id; ELSE rid := NEW.restaurant_id; END IF;
  UPDATE restaurants r
     SET rating       = COALESCE((SELECT round(avg(v.rating)::numeric, 1) FROM reviews v WHERE v.restaurant_id = rid), 0),
         review_count = (SELECT count(*) FROM reviews v WHERE v.restaurant_id = rid),
         updated_at   = now()
   WHERE r.id = rid;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reviews_refresh_rating
AFTER INSERT OR UPDATE OR DELETE ON reviews
FOR EACH ROW EXECUTE FUNCTION refresh_restaurant_rating();

-- ---------- seed ข้อมูลอ้างอิง ----------
INSERT INTO provinces (name_th) VALUES
  ('กรุงเทพมหานคร'),
  ('กระบี่'),
  ('กาญจนบุรี'),
  ('กาฬสินธุ์'),
  ('กำแพงเพชร'),
  ('ขอนแก่น'),
  ('จันทบุรี'),
  ('ฉะเชิงเทรา'),
  ('ชลบุรี'),
  ('ชัยนาท'),
  ('ชัยภูมิ'),
  ('ชุมพร'),
  ('เชียงราย'),
  ('เชียงใหม่'),
  ('ตรัง'),
  ('ตราด'),
  ('ตาก'),
  ('นครนายก'),
  ('นครปฐม'),
  ('นครพนม'),
  ('นครราชสีมา'),
  ('นครศรีธรรมราช'),
  ('นครสวรรค์'),
  ('นนทบุรี'),
  ('นราธิวาส'),
  ('น่าน'),
  ('บึงกาฬ'),
  ('บุรีรัมย์'),
  ('ปทุมธานี'),
  ('ประจวบคีรีขันธ์'),
  ('ปราจีนบุรี'),
  ('ปัตตานี'),
  ('พระนครศรีอยุธยา'),
  ('พะเยา'),
  ('พังงา'),
  ('พัทลุง'),
  ('พิจิตร'),
  ('พิษณุโลก'),
  ('เพชรบุรี'),
  ('เพชรบูรณ์'),
  ('แพร่'),
  ('ภูเก็ต'),
  ('มหาสารคาม'),
  ('มุกดาหาร'),
  ('แม่ฮ่องสอน'),
  ('ยโสธร'),
  ('ยะลา'),
  ('ร้อยเอ็ด'),
  ('ระนอง'),
  ('ระยอง'),
  ('ราชบุรี'),
  ('ลพบุรี'),
  ('ลำปาง'),
  ('ลำพูน'),
  ('เลย'),
  ('ศรีสะเกษ'),
  ('สกลนคร'),
  ('สงขลา'),
  ('สตูล'),
  ('สมุทรปราการ'),
  ('สมุทรสงคราม'),
  ('สมุทรสาคร'),
  ('สระแก้ว'),
  ('สระบุรี'),
  ('สิงห์บุรี'),
  ('สุโขทัย'),
  ('สุพรรณบุรี'),
  ('สุราษฎร์ธานี'),
  ('สุรินทร์'),
  ('หนองคาย'),
  ('หนองบัวลำภู'),
  ('อ่างทอง'),
  ('อำนาจเจริญ'),
  ('อุดรธานี'),
  ('อุตรดิตถ์'),
  ('อุทัยธานี'),
  ('อุบลราชธานี');

INSERT INTO cuisines (name) VALUES
  ('ไทย'),
  ('กาแฟ'),
  ('ของหวาน'),
  ('เบเกอรี่/คาเฟ่'),
  ('ฟาสต์ฟู้ด/ของทานเล่น'),
  ('อีสาน'),
  ('เอเชียนฟิวชัน'),
  ('อาหารทะเล'),
  ('อเมริกัน'),
  ('ยุโรป'),
  ('ไอศกรีม'),
  ('ก๋วยเตี๋ยว'),
  ('ญี่ปุ่น'),
  ('อาหารเหนือ'),
  ('อิตาเลียน'),
  ('อาหารมุสลิม'),
  ('จีน'),
  ('ฝรั่งเศส/บิสโทร'),
  ('ฮาลาล'),
  ('บาร์บีคิว'),
  ('เวียดนาม');

COMMIT;
