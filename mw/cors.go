package mw

import (
    "net/http"
    "strings"
)

// CORS อนุญาตเฉพาะ origin ที่อยู่ในรายชื่อ
func CORS(allowed []string, next http.Handler) http.Handler {
    ok := map[string]bool{}
    for _, o := range allowed {
        cleaned := strings.TrimSpace(o)
        if cleaned == "" {
            continue
        }
        ok[cleaned] = true
    }

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin != "" && (ok[origin] || ok["*"]) {
            h := w.Header()
            if ok["*"] {
                h.Set("Access-Control-Allow-Origin", "*")
            } else {
                h.Set("Access-Control-Allow-Origin", origin)
                h.Add("Vary", "Origin")
            }
            h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
            h.Set("Access-Control-Max-Age", "600")
        }

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}