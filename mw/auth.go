package mw

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"

    "foodjubb/auth"
)

type ctxKey int

const (
    memberIDKey ctxKey = iota
    roleKey
)

// MemberID คืนเลขสมาชิกจากตั๋วที่ตรวจแล้ว ok = false แปลว่าเป็น Guest (ไม่ได้ผ่าน RequireLogin / OptionalLogin)
func MemberID(ctx context.Context) (int64, bool) {
    id, ok := ctx.Value(memberIDKey).(int64)
    return id, ok
}

// Role คืนบทบาทจากตั๋ว ("member" หรือ "admin") ว่างถ้ายังไม่ได้ login
func Role(ctx context.Context) string {
    role, _ := ctx.Value(roleKey).(string)
    return role
}

// fail ตอบ error รูปแบบเดียวกับทุก endpoint ตามสัญญา: { "error": "ข้อความ" }
func fail(w http.ResponseWriter, status int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func bearer(r *http.Request) (string, bool) {
    header := r.Header.Get("Authorization")
    if !strings.HasPrefix(header, "Bearer ") {
        return "", false
    }
    return strings.TrimPrefix(header, "Bearer "), true
}

// RequireLogin คือพนักงานตรวจตั๋ว: ต้องมี Authorization: Bearer <token> ที่ตรวจผ่าน ไม่มี/ผิดตอบ 401
func RequireLogin(iss *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token, ok := bearer(r)
        if !ok {
            fail(w, http.StatusUnauthorized, "กรุณาเข้าสู่ระบบก่อน")
            return
        }
        id, role, err := iss.Parse(token)
        if err != nil {
            fail(w, http.StatusUnauthorized, "ตั๋วไม่ถูกต้องหรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
            return
        }
        ctx := context.WithValue(r.Context(), memberIDKey, id)
        ctx = context.WithValue(ctx, roleKey, role)
        next(w, r.WithContext(ctx))
    }
}

// OptionalLogin ใช้กับ endpoint สาธารณะที่ถ้ามีตั๋วจะตอบเพิ่ม (เช่น isWishlisted)
// Guest ไม่มีตั๋ว หรือตั๋วเสีย ก็ผ่านไปเป็น Guest ไม่ตอบ 401
func OptionalLogin(iss *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if token, ok := bearer(r); ok {
            if id, role, err := iss.Parse(token); err == nil {
                ctx := context.WithValue(r.Context(), memberIDKey, id)
                ctx = context.WithValue(ctx, roleKey, role)
                r = r.WithContext(ctx)
            }
        }
        next(w, r)
    }
}

// RequireRole ใช้ต่อจาก RequireLogin: ตั๋วถูกต้องแต่บทบาทไม่ตรง ตอบ 403
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if Role(r.Context()) != role {
            fail(w, http.StatusForbidden, "เฉพาะ "+role+" เท่านั้น")
            return
        }
        next(w, r)
    }
}
