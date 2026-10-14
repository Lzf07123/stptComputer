package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	RedisAddr       string
	AdminPassword   string
	QuestionSeconds int
	QuizCount       int
	UploadDir       string
	Title           string
}

func Load() Config {
	return Config{
		Port:            getenv("APP_PORT", "8080"),
		DatabaseURL:     getenv("DATABASE_URL", "postgres://quiz:quiz@localhost:5432/quiz?sslmode=disable"),
		RedisAddr:       getenv("REDIS_ADDR", "localhost:6379"),
		AdminPassword:   getenv("ADMIN_PASSWORD", "admin123"),
		QuestionSeconds: getenvInt("QUESTION_SECONDS", 20),
		QuizCount:       getenvInt("QUESTION_COUNT", 10),
		UploadDir:       getenv("UPLOAD_DIR", "uploads"),
		Title:           getenv("APP_TITLE", "计算机协会 · 招新答题"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
