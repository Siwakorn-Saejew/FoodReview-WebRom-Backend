#!/usr/bin/env python3
"""แปลง data/restaurants_100.csv เป็น seed.sql  รัน:  python3 scripts/make_seed.py

- ประเภทอาหารในไฟล์เป็นภาษาอังกฤษ แปลงเป็นชื่อไทยตาม CUISINE_MAP (ต้องตรงกับตาราง cuisines ใน schema.sql)
- ลิงก์รูปแบบ data:image (ฝังรูปในไฟล์) ข้ามไป เพราะยาวมากและทำให้รายการร้านหนัก
- ลิงก์ Facebook ที่ไม่มี https:// จะเติมให้
- คอลัมน์ michelin รูปแบบ "2022:Bib Gourmand,2023:Bib Gourmand" แยกเป็นตาราง restaurant_awards
"""
import csv, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, '..', 'data', 'restaurants_100.csv')
OUT = os.path.join(HERE, '..', 'seed.sql')

CUISINE_MAP = {
    'Thai Food': 'ไทย', 'Coffee Shop': 'กาแฟ', 'Desserts': 'ของหวาน',
    'Barcirerie/Cafe': 'เบเกอรี่/คาเฟ่', 'Fast Food/Snack': 'ฟาสต์ฟู้ด/ของทานเล่น',
    'Thai-Isan': 'อีสาน', 'Asian Fusion': 'เอเชียนฟิวชัน', 'Seafood': 'อาหารทะเล',
    'American': 'อเมริกัน', 'European': 'ยุโรป', 'Icecream': 'ไอศกรีม', 'Noodle': 'ก๋วยเตี๋ยว',
    'Japanese': 'ญี่ปุ่น', 'Thai northern food': 'อาหารเหนือ', 'Italian': 'อิตาเลียน',
    'Muslim Food': 'อาหารมุสลิม', 'Chinese': 'จีน', 'French/Bistro': 'ฝรั่งเศส/บิสโทร',
    'Halal and Kosher': 'ฮาลาล', 'Barbecue': 'บาร์บีคิว', 'Vietnam Food': 'เวียดนาม',
}

def q(v):
    """ค่าข้อความ -> ตัวอักษร SQL (ว่าง = NULL, ' ซ้ำสองครั้ง)"""
    v = (v or '').strip()
    return 'NULL' if v == '' else "'" + v.replace("'", "''") + "'"

def url(v):
    v = (v or '').strip()
    if v == '' or v.startswith('data:'):
        return ''
    if not re.match(r'^https?://', v):
        v = 'https://' + v
    return v

rows = list(csv.DictReader(open(SRC, encoding='utf-8-sig')))
out = []
out.append("""-- foodjubb seed: ร้านอาหาร %d ร้านจาก data/restaurants_100.csv + บัญชีทดลอง (รันหลัง schema.sql)
-- สร้างด้วย scripts/make_seed.py ห้ามแก้ไฟล์นี้ตรง ๆ ให้แก้ CSV แล้วรันสคริปต์ใหม่
-- บัญชีทดลองใช้รหัสผ่าน 1234  (demo@foodjubb.test = member, admin@foodjubb.test = admin)
-- รหัสผ่านถูกแฮชด้วย bcrypt ผ่าน pgcrypto ตอน seed (Go ตรวจด้วย bcrypt ได้เลย)
-- ใช้เฉพาะตอนพัฒนา ห้ามเอาบัญชีและรหัสผ่านนี้ขึ้น server จริง
-- ไม่มีรีวิวตั้งต้น: รีวิวต้องมาจากสมาชิกจริง ร้านที่ยังไม่มีรีวิวจะแสดงคะแนน 0 และ "ยังไม่มีรีวิว"

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------- สมาชิกทดลอง ----------
INSERT INTO members (id, email, password_hash, display_name, role)
VALUES (1, 'demo@foodjubb.test',  crypt('1234', gen_salt('bf', 10)), 'Demo',  'member'),
       (2, 'admin@foodjubb.test', crypt('1234', gen_salt('bf', 10)), 'Admin', 'admin');
SELECT setval(pg_get_serial_sequence('members', 'id'), (SELECT max(id) FROM members));
""" % len(rows))

vals, cuis, awards = [], [], []
skipped_images = 0
for i, r in enumerate(rows, start=1):
    image = url(r['image'])
    if r['image'].strip().startswith('data:'):
        skipped_images += 1
    vals.append("    (%d, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)" % (
        i, q(r['name']), q(r['nameTh']), q(r['province']), q(r['district']), q(r['address']),
        q(r['description']), q(r['phone']), q(r['openingHours']), q(r['menu']),
        q(image), q(r['imageCredit']), q(url(r['googleMapsLink'])), q(url(r['facebook']))))
    names = []
    for tok in r['cuisine'].split(','):
        tok = tok.strip()
        if tok == '':
            continue
        if tok not in CUISINE_MAP:
            sys.exit('ไม่รู้จักประเภทอาหาร %r ของร้าน %r  เพิ่มใน CUISINE_MAP และ schema.sql' % (tok, r['name']))
        th = CUISINE_MAP[tok]
        if th not in names:
            names.append(th)
    for pos, th in enumerate(names):
        cuis.append("    (%d, %s, %d)" % (i, q(th), pos))
    for part in (r['michelin'] or '').split(','):
        part = part.strip()
        if not part:
            continue
        m = re.match(r'^(\d{4}):(.+)$', part)
        if not m:
            sys.exit('รูปแบบ michelin ไม่ถูกต้อง %r ของร้าน %r' % (part, r['name']))
        awards.append("    (%d, %s, %s)" % (i, m.group(1), q(m.group(2))))

out.append("-- ---------- ร้านอาหาร (จังหวัดหาจากชื่อ) ----------")
out.append("""INSERT INTO restaurants (id, name, name_th, province_id, district, address, description, phone, opening_hours,
                         menu, image_url, image_credit, google_maps_link, facebook_url, created_at, updated_at)
SELECT v.id, v.name, v.name_th, (SELECT id FROM provinces WHERE name_th = v.province), v.district, v.address,
       v.description, v.phone, v.opening_hours, v.menu, v.image_url, v.image_credit, v.google_maps_link, v.facebook_url,
       now() - (%d - v.id) * interval '30 minutes', now()
FROM (VALUES""" % len(rows))
out.append(",\n".join(vals))
out.append(""") AS v(id, name, name_th, province, district, address, description, phone, opening_hours,
       menu, image_url, image_credit, google_maps_link, facebook_url);
SELECT setval(pg_get_serial_sequence('restaurants', 'id'), (SELECT max(id) FROM restaurants));
""")
out.append("-- ---------- ประเภทอาหาร (ลำดับแรก = ประเภทหลัก) ----------")
out.append("INSERT INTO restaurant_cuisines (restaurant_id, cuisine_id, position)\nSELECT v.rid, (SELECT id FROM cuisines WHERE name = v.cuisine), v.pos\nFROM (VALUES")
out.append(",\n".join(cuis))
out.append(") AS v(rid, cuisine, pos);\n")
if awards:
    out.append("-- ---------- รางวัล ----------")
    out.append("INSERT INTO restaurant_awards (restaurant_id, year, award) VALUES")
    out.append(",\n".join(awards) + ";\n")
out.append("COMMIT;\n")

open(OUT, 'w', encoding='utf-8').write("\n".join(out))
print('สร้าง seed.sql: %d ร้าน, %d รายการประเภท, %d รางวัล, ข้ามรูปที่ฝังในไฟล์ %d รูป' % (len(rows), len(cuis), len(awards), skipped_images))
