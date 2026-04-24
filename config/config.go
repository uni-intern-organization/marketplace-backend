package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server ServerConfig
	DB     DBConfig
	JWT    JWTConfig
	AES    AESConfig
	S3     S3Config
	OpenAI OpenAIConfig
}

// OpenAIConfig: API key only on the server, never in Vite env.
type OpenAIConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	EmbeddingModel string // e.g. text-embedding-3-small
}

type ServerConfig struct {
	Port string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	Secret      string
	ExpireHours int
}

type AESConfig struct {
	Key string // 32 bytes for AES-256, base64 or hex
}

type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

// Load reads .env from the current working directory (if present), then os.Getenv.
// Call this before using any value that may come from .env (e.g. OPENAI_API_KEY).
func Load() (*Config, error) {
	_ = godotenv.Load() // no error: file may be missing (e.g. Docker sets env only)
	port := getEnv("PORT", "8080")
	dbHost := getEnv("DB_HOST", "postgres")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "marketplace")

	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production-secret-key-32bytes!!")
	jwtExpire := getEnvInt("JWT_EXPIRE_HOURS", 24)
	aesKey := getEnv("AES_KEY", "0123456789abcdef0123456789abcdef") // 32 bytes

	s3Endpoint := getEnv("S3_ENDPOINT", "minio:9000")
	s3Region := getEnv("S3_REGION", "us-east-1")
	s3AccessKey := getEnv("S3_ACCESS_KEY_ID", "minioadmin")
	s3SecretKey := getEnv("S3_SECRET_ACCESS_KEY", "minioadmin")
	s3Bucket := getEnv("S3_BUCKET", "marketplace")
	s3UseSSL := getEnv("S3_USE_SSL", "false") == "true"

	openaiKey := getEnv("OPENAI_API_KEY", "")
	openaiBase := getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	openaiModel := getEnv("OPENAI_MODEL", "gpt-4o-mini")
	openaiEmbed := getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")

	return &Config{
		Server: ServerConfig{Port: port},
		DB: DBConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
			SSLMode:  "disable",
		},
		JWT: JWTConfig{
			Secret:      jwtSecret,
			ExpireHours: jwtExpire,
		},
		AES: AESConfig{Key: aesKey},
		S3: S3Config{
			Endpoint:        s3Endpoint,
			Region:          s3Region,
			AccessKeyID:     s3AccessKey,
			SecretAccessKey: s3SecretKey,
			Bucket:          s3Bucket,
			UseSSL:          s3UseSSL,
		},
		OpenAI: OpenAIConfig{
			APIKey:         openaiKey,
			BaseURL:        openaiBase,
			Model:          openaiModel,
			EmbeddingModel: openaiEmbed,
		},
	}, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
