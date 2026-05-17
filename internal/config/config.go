package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port string

	MongoURI string

	// Invitation gate used on signup; never persisted.
	AuthSignupPassword string

	JWTSecret   string
	JWTIssuer   string
	JWTAudience string

	UploadDir string
	BaseURL   string

	// Push notification scheduler (optional).
	FCMServiceAccountJSONPath string
	NotificationWindowMinutes int
}

func MustLoad() Config {
	cfg := Config{
		Port: os.Getenv("PORT"),

		MongoURI: os.Getenv("MONGODB_URI"),

		AuthSignupPassword: os.Getenv("AUTH_SIGNUP_PASSWORD"),

		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   os.Getenv("JWT_ISSUER"),
		JWTAudience: os.Getenv("JWT_AUDIENCE"),

		UploadDir: os.Getenv("UPLOAD_DIR"),
		BaseURL:   os.Getenv("BASE_URL"),
	}

	if cfg.Port == "" {
		cfg.Port = "8000"
	}
	if cfg.MongoURI == "" {
		panic("missing env: MONGODB_URI")
	}
	if cfg.AuthSignupPassword == "" {
		panic("missing env: AUTH_SIGNUP_PASSWORD")
	}
	if cfg.JWTSecret == "" {
		panic("missing env: JWT_SECRET")
	}
	if cfg.JWTIssuer == "" {
		cfg.JWTIssuer = "goal-tracker"
	}
	if cfg.JWTAudience == "" {
		cfg.JWTAudience = "goal-tracker"
	}
	if cfg.UploadDir == "" {
		cfg.UploadDir = "./storage"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:" + cfg.Port
	}

	cfg.FCMServiceAccountJSONPath = os.Getenv("FCM_SERVICE_ACCOUNT_JSON_PATH")
	if cfg.FCMServiceAccountJSONPath == "" {
		cfg.FCMServiceAccountJSONPath = ""
	}
	if mins := os.Getenv("NOTIFICATION_WINDOW_MINUTES"); mins != "" {
		// If parsing fails, keep default below.
		if parsed, err := strconv.Atoi(mins); err == nil {
			cfg.NotificationWindowMinutes = parsed
		}
	}
	if cfg.NotificationWindowMinutes == 0 {
		cfg.NotificationWindowMinutes = 10
	}

	return cfg
}

