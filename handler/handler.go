package handler

import (
    "encoding/json"
    "log"
    "net/http"
    "net/url"
    "strconv"

    "foodjubb/auth"
    "foodjubb/store"
)

type Handler struct {
    Store   *store.Store
    Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

// writeError ตอบ error รูปแบบเดียวกับทุก endpoint ตามสัญญา: { "error": "ข้อความ" }
func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
    log.Printf("internal error: %v", err)
    writeError(w, http.StatusInternalServerError, "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

// NotFound ตอบ JSON สำหรับ path ใต้ /api ที่ไม่มีอยู่ (หน้าเว็บจะได้ไม่เจอข้อความธรรมดาที่ไม่ใช่ JSON)
func NotFound(w http.ResponseWriter, r *http.Request) {
    writeError(w, http.StatusNotFound, "ไม่มี endpoint "+r.Method+" "+r.URL.Path)
}

func pathID(r *http.Request, name string) (int64, bool) {
    id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
    return id, err == nil && id > 0
}

// pageParams อ่าน page / limit จาก query string (page เริ่มที่ 1, limit ค่าเริ่มต้น 20 สูงสุด 100)
func pageParams(q url.Values) (page, limit int) {
    page, _ = strconv.Atoi(q.Get("page"))
    limit, _ = strconv.Atoi(q.Get("limit"))
    if page < 1 {
        page = 1
    }
    if limit < 1 {
        limit = 20
    }
    if limit > 100 {
        limit = 100
    }
    return page, limit
}
