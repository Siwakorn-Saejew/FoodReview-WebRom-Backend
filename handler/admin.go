package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "unicode/utf8"

    "foodjubb/mw"
    "foodjubb/store"
)

// ทุก handler ในไฟล์นี้ถูกครอบด้วย RequireLogin + RequireRole("admin") ใน main.go แล้ว
// จึงไม่ต้องเช็กบทบาทซ้ำในนี้ แต่ "ใครเป็นคนกด" ยังอ่านจากตั๋วผ่าน mw.MemberID เสมอ

// ---------- Dashboard ----------

func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
    st, err := h.Store.Stats(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, st)
}

// ---------- สมาชิก ----------

func (h *Handler) AdminMembers(w http.ResponseWriter, r *http.Request) {
    items, err := h.Store.ListMembers(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"results": items})
}

type roleReq struct {
    Role string `json:"role"`
}

// AdminSetRole เลื่อนหรือลดบทบาท ห้ามแก้ของตัวเอง (กัน admin คนสุดท้ายลดตัวเองจนไม่มีใครเข้าหลังบ้านได้)
func (h *Handler) AdminSetRole(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, "ไม่พบสมาชิก")
        return
    }
    var req roleReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    if req.Role != "member" && req.Role != "admin" {
        writeError(w, http.StatusBadRequest, "role ต้องเป็น admin หรือ member")
        return
    }
    if me, _ := mw.MemberID(r.Context()); me == id {
        writeError(w, http.StatusBadRequest, "เปลี่ยนบทบาทของตัวเองไม่ได้")
        return
    }
    m, err := h.Store.UpdateMemberRole(r.Context(), id, req.Role)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "ไม่พบสมาชิก")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}

// ---------- ร้านอาหาร ----------

// trimPtr ตัดช่องว่างหัวท้าย และทำให้ช่องว่างเปล่ากลายเป็น NULL ในฐานข้อมูล
func trimPtr(p *string) *string {
    if p == nil {
        return nil
    }
    s := strings.TrimSpace(*p)
    if s == "" {
        return nil
    }
    return &s
}

// ลิงก์ที่ admin กรอกต้องขึ้นต้นด้วย http:// หรือ https:// เท่านั้น (กัน javascript: ฯลฯ ที่หน้าเว็บเอาไปทำลิงก์)
func isHTTPURL(s string) bool {
    return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

const (
    maxCuisines     = 6
    maxCuisineRunes = 60 // ตรงกับ cuisines.name varchar(60)
)

// validateRestaurant ตรวจฝั่ง server ซ้ำอีกชั้น แม้ฟอร์ม React จะเช็กไปแล้ว เพราะ curl ไม่ผ่านฟอร์ม
// คืนข้อความ error (ว่าง = ผ่าน) และจัดข้อมูลให้เรียบร้อย (ตัดช่องว่าง รวมประเภทอาหารที่ซ้ำ)
func validateRestaurant(in *store.RestaurantInput) string {
    in.Name = strings.TrimSpace(in.Name)
    in.Province = strings.TrimSpace(in.Province)
    in.NameTh = trimPtr(in.NameTh)
    in.District = trimPtr(in.District)
    in.Description = trimPtr(in.Description)
    in.Address = trimPtr(in.Address)
    in.Phone = trimPtr(in.Phone)
    in.OpeningHours = trimPtr(in.OpeningHours)
    in.Menu = trimPtr(in.Menu)
    in.Image = trimPtr(in.Image)
    in.ImageCredit = trimPtr(in.ImageCredit)
    in.GoogleMapsLink = trimPtr(in.GoogleMapsLink)
    in.Facebook = trimPtr(in.Facebook)

    // ประเภทอาหาร: ใช้ cuisines (หลายประเภท) ถ้าไม่ส่งมาก็ใช้ cuisine ตัวเดียวแบบเดิม
    names := in.Cuisines
    if len(names) == 0 && strings.TrimSpace(in.Cuisine) != "" {
        names = []string{in.Cuisine}
    }
    seen := map[string]bool{}
    cleaned := []string{}
    for _, n := range names {
        if n = strings.TrimSpace(n); n != "" && !seen[n] {
            seen[n] = true
            cleaned = append(cleaned, n)
        }
    }
    in.Cuisines = cleaned
    in.Cuisine = ""

    switch {
    case in.Name == "":
        return "ต้องมีชื่อร้าน"
    case len(in.Cuisines) == 0:
        return "ต้องระบุประเภทอาหาร"
    case len(in.Cuisines) > maxCuisines:
        return "เลือกประเภทอาหารได้ไม่เกิน 6 ประเภท"
    case in.Province == "":
        return "ต้องเลือกจังหวัดให้ถูกต้อง"
    case in.Image != nil && !isHTTPURL(*in.Image):
        return "รูปภาพต้องเป็น URL ที่ขึ้นต้นด้วย http:// หรือ https://"
    case in.GoogleMapsLink != nil && !isHTTPURL(*in.GoogleMapsLink):
        return "ลิงก์แผนที่ต้องขึ้นต้นด้วย http:// หรือ https://"
    case in.Facebook != nil && !isHTTPURL(*in.Facebook):
        return "ลิงก์ Facebook ต้องขึ้นต้นด้วย http:// หรือ https://"
    }
    for _, n := range in.Cuisines {
        if utf8.RuneCountInString(n) > maxCuisineRunes {
            return "ชื่อประเภทอาหารยาวเกินไป"
        }
    }
    return ""
}

func (h *Handler) AdminGetRestaurant(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    a, err := h.Store.AdminGetRestaurant(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, a)
}

func (h *Handler) AdminCreateRestaurant(w http.ResponseWriter, r *http.Request) {
    var in store.RestaurantInput
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    if msg := validateRestaurant(&in); msg != "" {
        writeError(w, http.StatusBadRequest, msg)
        return
    }
    a, err := h.Store.CreateRestaurant(r.Context(), in)
    if errors.Is(err, store.ErrBadProvince) {
        writeError(w, http.StatusBadRequest, "ต้องเลือกจังหวัดให้ถูกต้อง")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) AdminUpdateRestaurant(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    var in store.RestaurantInput
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    if msg := validateRestaurant(&in); msg != "" {
        writeError(w, http.StatusBadRequest, msg)
        return
    }
    a, err := h.Store.UpdateRestaurant(r.Context(), id, in)
    switch {
    case errors.Is(err, store.ErrNotFound):
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    case errors.Is(err, store.ErrBadProvince):
        writeError(w, http.StatusBadRequest, "ต้องเลือกจังหวัดให้ถูกต้อง")
        return
    case err != nil:
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, a)
}

func (h *Handler) AdminDeleteRestaurant(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    err := h.Store.DeleteRestaurant(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent) // ลบแล้ว ไม่มีอะไรให้ส่งกลับ
}

// ---------- รีวิว ----------

func (h *Handler) AdminReviews(w http.ResponseWriter, r *http.Request) {
    items, err := h.Store.AdminListReviews(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"results": items})
}

func (h *Handler) AdminDeleteReview(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, "ไม่พบรีวิว")
        return
    }
    err := h.Store.AdminDeleteReview(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "ไม่พบรีวิว")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
