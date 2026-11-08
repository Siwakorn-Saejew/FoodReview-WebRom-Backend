package handler

import (
    "encoding/json"
    "errors"
    "io"
    "math"
    "mime/multipart"
    "net/http"
    "strconv"
    "strings"
    "unicode/utf8"

    "foodjubb/mw"
    "foodjubb/store"
)

// ---------- รีวิว ----------

const (
    maxReviewRunes = 2000
    maxAuthorRunes = 60

    maxPhotos      = 3                // รูปต่อรีวิว (ตรงกับ CHECK position BETWEEN 0 AND 2 ใน schema)
    maxPhotoBytes  = 5 << 20          // 5 MB ต่อรูป (ตรงกับ CHECK size_bytes ใน schema)
    maxUploadBytes = 16 << 20         // ทั้งคำขอ (3 รูป + ฟิลด์อื่น)
)

// ชนิดไฟล์ที่รับ ตรวจจากเนื้อไฟล์จริง (http.DetectContentType) ไม่เชื่อนามสกุลหรือ Content-Type ที่ผู้ใช้ส่งมา
var allowedPhotoTypes = map[string]bool{
    "image/jpeg": true,
    "image/png":  true,
    "image/webp": true,
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    exists, err := h.Store.RestaurantExists(r.Context(), id)
    if err != nil {
        internal(w, err)
        return
    }
    if !exists {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    page, limit := pageParams(r.URL.Query())
    items, total, err := h.Store.ListReviews(r.Context(), id, page, limit)
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"results": items, "page": page, "limit": limit, "total": total})
}

type reviewReq struct {
    Rating     *float64 `json:"rating"` // เป็นตัวชี้เพื่อแยก "ไม่ส่งมา" ออกจาก 0
    Text       string   `json:"text"`
    AuthorName string   `json:"authorName"`
}

// validateReview ตรวจตามลำดับที่สัญญา API กำหนด: ข้อความ > คะแนน > ชื่อ คืน (ข้อความที่ตัดช่องว่างแล้ว, คะแนน, ข้อความ error)
func validateReview(text string, rating *float64, authorName string) (string, int, string) {
    text = strings.TrimSpace(text)
    authorName = strings.TrimSpace(authorName)
    switch {
    case utf8.RuneCountInString(text) < 10:
        return "", 0, "รีวิวสั้นเกินไป"
    case utf8.RuneCountInString(text) > maxReviewRunes:
        return "", 0, "รีวิวยาวเกินไป"
    case rating == nil || *rating != math.Trunc(*rating) || *rating < 1 || *rating > 5:
        return "", 0, "คะแนนต้องอยู่ระหว่าง 1 ถึง 5"
    case authorName == "":
        return "", 0, "กรุณาใส่ชื่อ"
    case utf8.RuneCountInString(authorName) > maxAuthorRunes:
        return "", 0, "ชื่อยาวเกินไป"
    }
    return text, int(*rating), ""
}

// checkPhoto ตรวจรูป 1 ไฟล์: ขนาด และชนิดจากเนื้อไฟล์จริง คืนข้อความ error (ว่าง = ผ่าน) และชนิดที่ตรวจพบ
func checkPhoto(data []byte) (contentType, msg string) {
    if len(data) == 0 {
        return "", "ไฟล์รูปว่างเปล่า"
    }
    if len(data) > maxPhotoBytes {
        return "", "รูปใหญ่เกินไป (ไม่เกิน 5 MB ต่อรูป)"
    }
    head := data
    if len(head) > 512 {
        head = head[:512]
    }
    contentType = http.DetectContentType(head)
    if !allowedPhotoTypes[contentType] {
        return "", "ไฟล์รูปต้องเป็น JPG, PNG หรือ WebP"
    }
    return contentType, ""
}

// readReviewRequest อ่านคำขอรีวิวได้ 2 แบบ
//   - JSON: { rating, text, authorName }            (ไม่มีรูป ตามสัญญาเดิม)
//   - multipart/form-data: rating, text, authorName + ไฟล์ในฟิลด์ photos (ซ้ำได้ไม่เกิน 3 ไฟล์)
// คืนข้อความ error (ว่าง = อ่านได้) ส่วนรูปจะถูกตรวจหลังตรวจข้อความรีวิวแล้ว ตามลำดับ error ในสัญญา
func readReviewRequest(w http.ResponseWriter, r *http.Request) (req reviewReq, files []*multipart.FileHeader, msg string) {
    if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            return req, nil, "อ่านข้อมูล JSON ไม่ออก"
        }
        return req, nil, ""
    }

    r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes) // กันส่งไฟล์ใหญ่มหาศาลมากินหน่วยความจำ
    if err := r.ParseMultipartForm(8 << 20); err != nil {
        return req, nil, "ส่งรูปไม่สำเร็จ หรือไฟล์ใหญ่เกินไป (รวมไม่เกิน 16 MB)"
    }
    req.Text = r.FormValue("text")
    req.AuthorName = r.FormValue("authorName")
    if v, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("rating")), 64); err == nil {
        req.Rating = &v
    }
    return req, r.MultipartForm.File["photos"], ""
}

// CreateReview: POST /api/restaurants/{id}/reviews (เฉพาะสมาชิก ครอบด้วย RequireLogin ใน main.go แล้ว)
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
    restaurantID, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    memberID, ok := mw.MemberID(r.Context()) // ใครเป็นคนรีวิว อ่านจากตั๋วเท่านั้น ไม่รับจาก body
    if !ok {
        writeError(w, http.StatusUnauthorized, "กรุณาเข้าสู่ระบบก่อน")
        return
    }
    exists, err := h.Store.RestaurantExists(r.Context(), restaurantID)
    if err != nil {
        internal(w, err)
        return
    }
    if !exists {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }

    req, files, msg := readReviewRequest(w, r)
    if r.MultipartForm != nil {
        defer r.MultipartForm.RemoveAll() // ลบไฟล์ชั่วคราวที่ Go พักไว้บนดิสก์
    }
    if msg != "" {
        writeError(w, http.StatusBadRequest, msg)
        return
    }
    text, rating, msg := validateReview(req.Text, req.Rating, req.AuthorName)
    if msg != "" {
        writeError(w, http.StatusBadRequest, msg)
        return
    }

    // ตรวจรูปทีละไฟล์ (หลังตรวจข้อความรีวิวผ่านแล้ว)
    if len(files) > maxPhotos {
        writeError(w, http.StatusBadRequest, "แนบรูปได้ไม่เกิน 3 รูป")
        return
    }
    photos := []store.PhotoInput{}
    for _, fh := range files {
        f, err := fh.Open()
        if err != nil {
            writeError(w, http.StatusBadRequest, "อ่านไฟล์รูปไม่ได้")
            return
        }
        data, err := io.ReadAll(io.LimitReader(f, maxPhotoBytes+1)) // อ่านเกินเพดานไป 1 ไบต์ไว้จับรูปที่ใหญ่เกิน
        f.Close()
        if err != nil {
            writeError(w, http.StatusBadRequest, "อ่านไฟล์รูปไม่ได้")
            return
        }
        contentType, msg := checkPhoto(data)
        if msg != "" {
            writeError(w, http.StatusBadRequest, msg)
            return
        }
        photos = append(photos, store.PhotoInput{ContentType: contentType, Data: data})
    }

    review, err := h.Store.CreateReview(r.Context(), restaurantID, memberID, rating, text, strings.TrimSpace(req.AuthorName), photos)
    switch {
    case errors.Is(err, store.ErrReviewExists):
        writeError(w, http.StatusConflict, "คุณรีวิวร้านนี้ไปแล้ว")
        return
    case errors.Is(err, store.ErrNotFound):
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    case err != nil:
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, review)
}

// GetPhoto: GET /api/photos/{id} (สาธารณะ ใช้เป็น src ของ <img>) ส่งไฟล์รูปที่เก็บไว้ในฐานข้อมูล
func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, "ไม่พบรูป")
        return
    }
    contentType, data, err := h.Store.GetPhoto(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "ไม่พบรูป")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("X-Content-Type-Options", "nosniff") // บอกเบราว์เซอร์ให้เชื่อชนิดที่เราส่ง ห้ามเดาเอง
    w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // รูปไม่เคยถูกแก้ แคชได้ยาว
    w.WriteHeader(http.StatusOK)
    w.Write(data)
}
