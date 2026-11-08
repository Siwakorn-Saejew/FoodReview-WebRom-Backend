package handler

import (
    "encoding/json"
    "errors"
    "net/http"

    "foodjubb/mw"
    "foodjubb/store"
)

// ---------- ร้านที่อยากไปกิน (เฉพาะสมาชิก แยกรายการด้วยตั๋ว) ----------
// ทุก handler ในไฟล์นี้ถูกครอบด้วย RequireLogin ใน main.go แล้ว

func (h *Handler) ListWishlist(w http.ResponseWriter, r *http.Request) {
    memberID, _ := mw.MemberID(r.Context())
    items, err := h.Store.ListWishlist(r.Context(), memberID)
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"results": items, "total": len(items)})
}

type wishlistReq struct {
    RestaurantID int64 `json:"restaurantId"`
}

func (h *Handler) AddWishlist(w http.ResponseWriter, r *http.Request) {
    memberID, _ := mw.MemberID(r.Context())
    var req wishlistReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "อ่านข้อมูล JSON ไม่ออก")
        return
    }
    if req.RestaurantID <= 0 {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    addedAt, created, err := h.Store.AddWishlist(r.Context(), memberID, req.RestaurantID)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    status := http.StatusCreated
    if !created {
        status = http.StatusOK // เพิ่มไปแล้ว ตอบข้อมูลเดิม กดซ้ำได้ไม่พัง
    }
    writeJSON(w, status, map[string]any{"restaurantId": req.RestaurantID, "addedAt": addedAt})
}

func (h *Handler) RemoveWishlist(w http.ResponseWriter, r *http.Request) {
    memberID, _ := mw.MemberID(r.Context())
    // เลขไม่ถูกต้องก็ตอบ 204 เหมือนกัน เพราะผลลัพธ์คือ "ร้านนี้ไม่อยู่ในรายการ" อยู่แล้ว
    if id, ok := pathID(r, "restaurantId"); ok {
        if err := h.Store.RemoveWishlist(r.Context(), memberID, id); err != nil {
            internal(w, err)
            return
        }
    }
    w.WriteHeader(http.StatusNoContent)
}
