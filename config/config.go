package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server       ServerConfig
	DB           DBConfig
	JWT          JWTConfig
	AES          AESConfig
	S3           S3Config
	Billing      BillingConfig
	SMTP         SMTPConfig
	Push         PushConfig
	Payment      PaymentConfig
	App          AppConfig
	RateLimit    RateLimitConfig
}

type AppConfig struct {
	FrontendURL string
	AppName     string
}

type RateLimitConfig struct {
	MaxFailedLogins int
	LockoutMinutes  int
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	Enabled  bool
}

type PushConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	Enabled         bool
}

type PaymentConfig struct {
	Provider      string // demo, kaspi, stripe
	KaspiAPIKey   string
	StripeSecret  string
	StripeWebhook string
}

type BillingConfig struct {
	FreelancePlatformFeePercent int
	VacancyTierBasicKZT         int
	VacancyTierStandardKZT      int
	VacancyTierPremiumKZT       int
	PlanStarterKZT              int
	PlanBusinessKZT             int
	PlanCorporateKZT            int
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
	Secret            string
	ExpireHours       int
	RefreshExpireDays int
}

type AESConfig struct {
	Key string
}

type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

func Load() (*Config, error) {
	loadDotEnv()

	port := getEnv("PORT", "8080")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5433")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "marketplace")

	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production-secret-key-32bytes!!")
	jwtExpire := getEnvInt("JWT_EXPIRE_HOURS", 1)
	refreshDays := getEnvInt("JWT_REFRESH_EXPIRE_DAYS", 30)
	aesKey := getEnv("AES_KEY", "0123456789abcdef0123456789abcdef")

	s3Endpoint := getEnv("S3_ENDPOINT", "localhost:9000")
	s3Region := getEnv("S3_REGION", "us-east-1")
	s3AccessKey := getEnv("S3_ACCESS_KEY_ID", "minioadmin")
	s3SecretKey := getEnv("S3_SECRET_ACCESS_KEY", "minioadmin")
	s3Bucket := getEnv("S3_BUCKET", "marketplace")
	s3UseSSL := getEnv("S3_USE_SSL", "false") == "true"

	freelanceFee := getEnvInt("FREELANCE_PLATFORM_FEE_PERCENT", 15)
	tierBasic := getEnvInt("VACANCY_TIER_BASIC_KZT", 5000)
	tierStandard := getEnvInt("VACANCY_TIER_STANDARD_KZT", 15000)
	tierPremium := getEnvInt("VACANCY_TIER_PREMIUM_KZT", 40000)

	smtpHost := getEnv("SMTP_HOST", "")
	smtpPort := getEnvInt("SMTP_PORT", 587)
	smtpUser := getEnv("SMTP_USER", "")
	smtpPass := getEnv("SMTP_PASSWORD", "")
	smtpFrom := getEnv("SMTP_FROM", "noreply@steppy.kz")

	return &Config{
		Server: ServerConfig{Port: port},
		App: AppConfig{
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
			AppName:     getEnv("APP_NAME", "Steppy Marketplace"),
		},
		RateLimit: RateLimitConfig{
			MaxFailedLogins: getEnvInt("MAX_FAILED_LOGINS", 5),
			LockoutMinutes:  getEnvInt("LOCKOUT_MINUTES", 15),
		},
		SMTP: SMTPConfig{
			Host: smtpHost, Port: smtpPort, User: smtpUser, Password: smtpPass, From: smtpFrom,
			Enabled: smtpHost != "",
		},
		Push: PushConfig{
			VAPIDPublicKey:  getEnv("VAPID_PUBLIC_KEY", ""),
			VAPIDPrivateKey: getEnv("VAPID_PRIVATE_KEY", ""),
			Enabled:         getEnv("VAPID_PUBLIC_KEY", "") != "",
		},
		Payment: PaymentConfig{
			Provider:     getEnv("PAYMENT_PROVIDER", "demo"),
			KaspiAPIKey:  getEnv("KASPI_API_KEY", ""),
			StripeSecret: getEnv("STRIPE_SECRET_KEY", ""),
		},
		Billing: BillingConfig{
			FreelancePlatformFeePercent: freelanceFee,
			VacancyTierBasicKZT:         tierBasic,
			VacancyTierStandardKZT:      tierStandard,
			VacancyTierPremiumKZT:       tierPremium,
			PlanStarterKZT:              getEnvInt("PLAN_STARTER_KZT", 30000),
			PlanBusinessKZT:             getEnvInt("PLAN_BUSINESS_KZT", 80000),
			PlanCorporateKZT:            getEnvInt("PLAN_CORPORATE_KZT", 200000),
		},
		DB: DBConfig{
			Host: dbHost, Port: dbPort, User: dbUser, Password: dbPassword, DBName: dbName, SSLMode: "disable",
		},
		JWT: JWTConfig{
			Secret: jwtSecret, ExpireHours: jwtExpire, RefreshExpireDays: refreshDays,
		},
		AES: AESConfig{Key: aesKey},
		S3: S3Config{
			Endpoint: s3Endpoint, Region: s3Region, AccessKeyID: s3AccessKey,
			SecretAccessKey: s3SecretKey, Bucket: s3Bucket, UseSSL: s3UseSSL,
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

func loadDotEnv() {
	paths := []string{".env", "marketplace-backend/.env"}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Printf("config: skip %s: %v", path, err)
			continue
		}
		log.Printf("config: loaded %s", path)
		return
	}
	log.Println("config: no .env file found, using defaults and OS environment")
}

func (c *Config) LogSummary() {
	log.Printf("config: server :%s | db %s@%s:%s/%s | s3 %s bucket=%s | smtp=%v | payment=%s",
		c.Server.Port, c.DB.User, c.DB.Host, c.DB.Port, c.DB.DBName, c.S3.Endpoint, c.S3.Bucket,
		c.SMTP.Enabled, c.Payment.Provider)
}
