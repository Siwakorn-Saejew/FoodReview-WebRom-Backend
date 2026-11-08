package handler

import (
    "errors"
    "net/http"
    "strings"

    "foodjubb/mw"
    "foodjubb/store"
)

const msgRestaurantNotFound = "ไม่พบร้านอาหาร"

// ---------- ร้านอาหาร (สาธารณะ: Guest ดูได้ ถ้าแนบตั๋วจะตอบ isWishlisted ด้วย) ----------

// SearchRestaurants: GET /api/restaurants?q=&province=&cuisine=&sort=&page=&limit=
func (h *Handler) SearchRestaurants(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    page, limit := pageParams(q)
    memberID, _ := mw.MemberID(r.Context()) // 0 ถ้าเป็น Guest
    items, total, err := h.Store.SearchRestaurants(r.Context(), store.SearchParams{
        Q:        q.Get("q"),
        Province: strings.TrimSpace(q.Get("province")),
        Cuisine:  strings.TrimSpace(q.Get("cuisine")),
        Sort:     q.Get("sort"),
        Page:     page,
        Limit:    limit,
        MemberID: memberID,
    })
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"results": items, "page": page, "limit": limit, "total": total})
}

func (h *Handler) GetRestaurant(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    memberID, _ := mw.MemberID(r.Context())
    d, err := h.Store.GetRestaurant(r.Context(), id, memberID)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, msgRestaurantNotFound)
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, d)
}
