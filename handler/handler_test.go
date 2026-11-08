package handler

import (
    "net/url"
    "strings"
    "testing"

    "foodjubb/store"
)

func TestPageParamsDefaultsAndLimits(t *testing.T) {
    cases := []struct {
        query       string
        page, limit int
    }{
        {"", 1, 20},
        {"page=3&limit=8", 3, 8},
        {"page=0&limit=0", 1, 20},
        {"page=-5&limit=-1", 1, 20},
        {"page=abc&limit=xyz", 1, 20},
        {"limit=9999", 1, 100},
    }
    for _, c := range cases {
        q, _ := url.ParseQuery(c.query)
        page, limit := pageParams(q)
        if page != c.page || limit != c.limit {
            t.Errorf("%q: got page=%d limit=%d, want %d %d", c.query, page, limit, c.page, c.limit)
        }
    }
}

func TestValidateRestaurant(t *testing.T) {
    ok := func() store.RestaurantInput {
        return store.RestaurantInput{Name: "Ramen", Cuisines: []string{"ญี่ปุ่น"}, Province: "กรุงเทพมหานคร"}
    }

    in := ok()
    if msg := validateRestaurant(&in); msg != "" {
        t.Fatalf("ข้อมูลถูกต้องต้องผ่าน: %s", msg)
    }

    in = ok()
    in.Name = "   "
    if msg := validateRestaurant(&in); msg != "ต้องมีชื่อร้าน" {
        t.Errorf("ชื่อว่าง: got %q", msg)
    }

    in = ok()
    in.Cuisines = nil
    if msg := validateRestaurant(&in); msg != "ต้องระบุประเภทอาหาร" {
        t.Errorf("ไม่มีประเภท: got %q", msg)
    }

    in = ok()
    in.Province = ""
    if msg := validateRestaurant(&in); msg == "" {
        t.Error("จังหวัดว่างต้องไม่ผ่าน")
    }

    for _, bad := range []string{"ftp://x", "javascript:alert(1)", "www.facebook.com/x"} {
        in = ok()
        in.Image = &bad
        if msg := validateRestaurant(&in); msg == "" {
            t.Errorf("รูปที่ไม่ใช่ http/https ต้องไม่ผ่าน: %s", bad)
        }
        in = ok()
        in.Facebook = &bad
        if msg := validateRestaurant(&in); msg == "" {
            t.Errorf("ลิงก์ Facebook ที่ไม่ใช่ http/https ต้องไม่ผ่าน: %s", bad)
        }
    }
}

func TestValidateRestaurantCuisines(t *testing.T) {
    // ส่ง cuisine ตัวเดียวแบบเดิมก็ใช้ได้
    in := store.RestaurantInput{Name: "A", Cuisine: " ไทย ", Province: "ภูเก็ต"}
    if msg := validateRestaurant(&in); msg != "" || len(in.Cuisines) != 1 || in.Cuisines[0] != "ไทย" {
        t.Errorf("cuisine เดี่ยว: msg=%q cuisines=%v", msg, in.Cuisines)
    }

    // หลายประเภท: ตัดช่องว่าง ทิ้งค่าว่างและค่าซ้ำ ลำดับแรกคือประเภทหลัก
    in = store.RestaurantInput{Name: "A", Cuisines: []string{" ไทย", "", "ของหวาน", "ไทย  "}, Province: "ภูเก็ต"}
    if msg := validateRestaurant(&in); msg != "" {
        t.Fatalf("หลายประเภท: %s", msg)
    }
    if len(in.Cuisines) != 2 || in.Cuisines[0] != "ไทย" || in.Cuisines[1] != "ของหวาน" {
        t.Errorf("ต้องเหลือ [ไทย ของหวาน] ได้ %v", in.Cuisines)
    }

    // มากเกินไป
    in = store.RestaurantInput{Name: "A", Cuisines: []string{"1", "2", "3", "4", "5", "6", "7"}, Province: "ภูเก็ต"}
    if msg := validateRestaurant(&in); msg == "" {
        t.Error("เกิน 6 ประเภทต้องไม่ผ่าน")
    }
}

func TestCheckPhoto(t *testing.T) {
    jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 100)...)
    png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 100)...)
    text := []byte("<script>alert(1)</script> นี่ไม่ใช่รูป")

    if ct, msg := checkPhoto(jpeg); msg != "" || ct != "image/jpeg" {
        t.Errorf("jpeg: ct=%q msg=%q", ct, msg)
    }
    if ct, msg := checkPhoto(png); msg != "" || ct != "image/png" {
        t.Errorf("png: ct=%q msg=%q", ct, msg)
    }
    if _, msg := checkPhoto(text); msg != "ไฟล์รูปต้องเป็น JPG, PNG หรือ WebP" {
        t.Errorf("ไฟล์ที่ไม่ใช่รูปต้องถูกปฏิเสธ: %q", msg)
    }
    if _, msg := checkPhoto(nil); msg == "" {
        t.Error("ไฟล์ว่างต้องถูกปฏิเสธ")
    }
    big := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, maxPhotoBytes)...)
    if _, msg := checkPhoto(big); msg != "รูปใหญ่เกินไป (ไม่เกิน 5 MB ต่อรูป)" {
        t.Errorf("รูปเกิน 5 MB ต้องถูกปฏิเสธ: %q", msg)
    }
}

func TestValidateReview(t *testing.T) {
    five := 5.0
    nine := 9.0
    frac := 4.5
    long := strings.Repeat("ก", 12)

    cases := []struct {
        name   string
        text   string
        rating *float64
        author string
        want   string
    }{
        {"ผ่าน", long, &five, "สมชาย", ""},
        {"สั้นเกินไป", "สั้น", &five, "สมชาย", "รีวิวสั้นเกินไป"},
        {"คะแนนเกิน", long, &nine, "สมชาย", "คะแนนต้องอยู่ระหว่าง 1 ถึง 5"},
        {"คะแนนเป็นทศนิยม", long, &frac, "สมชาย", "คะแนนต้องอยู่ระหว่าง 1 ถึง 5"},
        {"ไม่ส่งคะแนน", long, nil, "สมชาย", "คะแนนต้องอยู่ระหว่าง 1 ถึง 5"},
        {"ชื่อว่าง", long, &five, "  ", "กรุณาใส่ชื่อ"},
    }
    for _, c := range cases {
        _, _, msg := validateReview(c.text, c.rating, c.author)
        if msg != c.want {
            t.Errorf("%s: got %q, want %q", c.name, msg, c.want)
        }
    }
}
