package auth

import (
    "testing"
    "time"
)

func newIssuer(ttl time.Duration) *Issuer {
    return &Issuer{Secret: []byte("abcdefghijklmnopqrstuvwxyz123456"), TTL: ttl}
}

func TestIssueThenParseReturnsSameMemberAndRole(t *testing.T) {
    iss := newIssuer(time.Hour)
    token, err := iss.Issue(42, "admin")
    if err != nil {
        t.Fatalf("issue: %v", err)
    }
    id, role, err := iss.Parse(token)
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    if id != 42 || role != "admin" {
        t.Fatalf("got id=%d role=%q, want 42 admin", id, role)
    }
}

func TestParseRejectsTokenSignedWithOtherSecret(t *testing.T) {
    token, _ := newIssuer(time.Hour).Issue(1, "member")
    other := &Issuer{Secret: []byte("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"), TTL: time.Hour}
    if _, _, err := other.Parse(token); err == nil {
        t.Fatal("ตั๋วที่เซ็นด้วยกุญแจอื่นต้องถูกปฏิเสธ")
    }
}

func TestParseRejectsExpiredToken(t *testing.T) {
    iss := newIssuer(-time.Minute) // หมดอายุไปแล้วตั้งแต่ออก
    token, _ := iss.Issue(1, "member")
    if _, _, err := iss.Parse(token); err == nil {
        t.Fatal("ตั๋วหมดอายุต้องถูกปฏิเสธ")
    }
}

func TestParseRejectsGarbage(t *testing.T) {
    if _, _, err := newIssuer(time.Hour).Parse("not-a-token"); err == nil {
        t.Fatal("ข้อความมั่วต้องถูกปฏิเสธ")
    }
}

func TestPasswordHashAndCheck(t *testing.T) {
    hash, err := HashPassword("1234")
    if err != nil {
        t.Fatalf("hash: %v", err)
    }
    if !CheckPassword(hash, "1234") {
        t.Fatal("รหัสถูกต้องต้องผ่าน")
    }
    if CheckPassword(hash, "1235") {
        t.Fatal("รหัสผิดต้องไม่ผ่าน")
    }
}
