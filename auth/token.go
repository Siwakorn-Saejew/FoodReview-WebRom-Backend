package auth

import (
    "errors"
    "strconv"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    Role string `json:"role"`
    jwt.RegisteredClaims
}

type Issuer struct {
    Secret []byte
    TTL    time.Duration
}

// Issue ออกตั๋วให้สมาชิกหนึ่งคน
func (i *Issuer) Issue(memberID int64, role string) (string, error) {
    now := time.Now()
    claims := Claims{
        Role: role,
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   strconv.FormatInt(memberID, 10),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(i.TTL)),
        },
    }
    return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.Secret)
}

// Parse ตรวจตั๋ว ถ้าผ่านคืนเลขสมาชิกกับบทบาท
func (i *Issuer) Parse(tokenString string) (int64, string, error) {
    var c Claims
    _, err := jwt.ParseWithClaims(tokenString, &c,
        func(t *jwt.Token) (any, error) { return i.Secret, nil },
        jwt.WithValidMethods([]string{"HS256"}),
        jwt.WithExpirationRequired(),
    )
    if err != nil {
        return 0, "", err
    }
    id, err := strconv.ParseInt(c.Subject, 10, 64)
    if err != nil {
        return 0, "", errors.New("bad subject")
    }
    return id, c.Role, nil
}