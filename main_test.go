package main

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadEnvLoadsKeyValuePairsFromDotEnv(t *testing.T) {
    tempDir := t.TempDir()
    envPath := filepath.Join(tempDir, ".env")

    content := "JWT_SECRET=abcdefghijklmnopqrstuvwxyz123456\nDATABASE_URL=postgres://localhost:5432/test\n"
    if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
        t.Fatalf("write env file: %v", err)
    }

    oldSecret, hadSecret := os.LookupEnv("JWT_SECRET")
    oldDB, hadDB := os.LookupEnv("DATABASE_URL")
    os.Unsetenv("JWT_SECRET")
    os.Unsetenv("DATABASE_URL")
    t.Cleanup(func() {
        if hadSecret {
            os.Setenv("JWT_SECRET", oldSecret)
        } else {
            os.Unsetenv("JWT_SECRET")
        }
        if hadDB {
            os.Setenv("DATABASE_URL", oldDB)
        } else {
            os.Unsetenv("DATABASE_URL")
        }
    })

    if err := loadEnv(envPath); err != nil {
        t.Fatalf("load env: %v", err)
    }

    if got := os.Getenv("JWT_SECRET"); got != "abcdefghijklmnopqrstuvwxyz123456" {
        t.Fatalf("JWT_SECRET = %q, want %q", got, "abcdefghijklmnopqrstuvwxyz123456")
    }
    if got := os.Getenv("DATABASE_URL"); got != "postgres://localhost:5432/test" {
        t.Fatalf("DATABASE_URL = %q, want %q", got, "postgres://localhost:5432/test")
    }
}
