package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "unicode/utf8"

    "foodjubb/auth"
    "foodjubb/mw"
    "foodjubb/store"
)

// รหัสผ่านอย่างน้อย 4 ตัวตามที่หน้าเว็บและสัญญา API กำหนด 
const minPasswordLen = 4

type registerReq struct {
    Email       string `json:"email"`
    Password    string `json:"password"`
    DisplayName string `json:"displayName"`
}

type loginReq struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    var req registerReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    email := strings.ToLower(strings.TrimSpace(req.Email))
    name := strings.TrimSpace(req.DisplayName)
    switch {
    case !strings.Contains(email, "@") || len(email) > 255:
        writeError(w, http.StatusBadRequest, "รูปแบบอีเมลไม่ถูกต้อง")
        return
    case utf8.RuneCountInString(req.Password) < minPasswordLen || len(req.Password) > 72:
        writeError(w, http.StatusBadRequest, "รหัสผ่านต้องยาว 4 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์")
        return
    case name == "" || utf8.RuneCountInString(name) > 50:
        writeError(w, http.StatusBadRequest, "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
        return
    }
    hash, err := auth.HashPassword(req.Password)
    if err != nil {
        internal(w, err)
        return
    }
    m, err := h.Store.CreateMember(r.Context(), email, hash, name)
    if errors.Is(err, store.ErrEmailTaken) {
        writeError(w, http.StatusConflict, "อีเมลนี้ถูกใช้แล้ว")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, m)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    var req loginReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    email := strings.ToLower(strings.TrimSpace(req.Email))
    m, err := h.Store.MemberByEmail(r.Context(), email)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusUnauthorized, "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    if !auth.CheckPassword(m.PasswordHash, req.Password) {
        // ข้อความเดียวกับกรณีไม่พบอีเมล คนร้ายจะได้ไม่รู้ว่าอีเมลไหนมีบัญชีอยู่
        writeError(w, http.StatusUnauthorized, "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
        return
    }
    token, err := h.Tickets.Issue(m.ID, m.Role)
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"token": token, "member": m})
}

// Me ตอบข้อมูลของคนที่ถือตั๋ว เลขสมาชิกมาจากตั๋วที่ RequireLogin ตรวจแล้วเท่านั้น
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
    id, ok := mw.MemberID(r.Context())
    if !ok {
        writeError(w, http.StatusUnauthorized, "กรุณาเข้าสู่ระบบก่อน")
        return
    }
    m, err := h.Store.MemberByID(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusUnauthorized, "ไม่พบบัญชีนี้แล้ว กรุณาเข้าสู่ระบบใหม่")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}
