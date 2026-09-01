// Package config handles application configuration loading and validation.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/secrets"
)

// Config holds all application configuration.
type Config struct {
	App            AppConfig
	Server         ServerConfig
	Postgres       PostgresConfig
	Redis          RedisConfig
	JWT            JWTConfig
	MTLS           MTLSConfig
	CORS           CORSConfig
	PythonAI       PythonAIConfig
	RateLimit      RateLimitConfig
	Email          EmailConfig
	CompaniesHouse CompaniesHouseConfig
	FrontendURL    string
}

// EmailConfig holds email service settings.
type EmailConfig struct {
	APIKey    string
	FromEmail string
	FromName  string
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled       bool
	RequestsPerIP int           // Max requests per IP per window
	Window        time.Duration // Time window for rate limiting
	BurstSize     int           // Allow burst above limit temporarily
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowCredentials bool
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Env      string
	Name     string
	Debug    bool
	LogLevel string
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// PostgresConfig holds database connection settings.
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	PoolMin  int
	PoolMax  int
}

// DSN returns the PostgreSQL connection string.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// URL returns the PostgreSQL connection URL.
func (c PostgresConfig) URL() string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host       string
	Port       int
	Password   string
	DB         int
	TLSEnabled bool
	PoolSize   int
}

// Addr returns the Redis address.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	SecretKey           string
	AccessTokenExpire   time.Duration
	RefreshTokenExpire  time.Duration
	Issuer              string
}

// MTLSConfig holds mTLS certificate settings.
type MTLSConfig struct {
	Enabled    bool
	CACert     string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
}

// PythonAIConfig holds Python AI service connection settings.
type PythonAIConfig struct {
	BaseURL string
	Timeout time.Duration
}

// CompaniesHouseConfig holds UK Companies House API settings.
type CompaniesHouseConfig struct {
	APIKey   string
	BaseURL  string
	Timeout  time.Duration
	CacheTTL time.Duration
}

// Load reads configuration from environment variables.
// It optionally loads a .env file if present.
// When SECRETS_FROM_KMS=true, sensitive values are fetched from Alibaba Cloud KMS.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{}

	// Initialize KMS client if secrets should be fetched from KMS
	var kmsClient *secrets.Client
	if secrets.IsKMSEnabled() {
		region := getEnv("KMS_REGION", "eu-west-1")
		var err error
		kmsClient, err = secrets.NewClient(region)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize KMS client: %w", err)
		}
		log.Info().Str("region", region).Msg("KMS secret fetching enabled")
	}

	// App config
	cfg.App = AppConfig{
		Env:      getEnv("APP_ENV", "development"),
		Name:     getEnv("APP_NAME", "accountant-crm"),
		Debug:    getEnvBool("APP_DEBUG", true),
		LogLevel: getEnv("LOG_LEVEL", "debug"),
	}

	// Server config
	cfg.Server = ServerConfig{
		Host:            getEnv("GO_HOST", "0.0.0.0"),
		Port:            getEnvInt("GO_PORT", 8080),
		ReadTimeout:     getEnvDuration("GO_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    getEnvDuration("GO_WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvDuration("GO_SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	// Postgres config
	// Note: Neon has a 10-connection limit, so pool max should be ≤8 to leave headroom
	postgresPassword := getEnv("POSTGRES_PASSWORD", "")
	if kmsClient != nil && postgresPassword == "" {
		secretName := getEnv("KMS_POSTGRES_PASSWORD_SECRET", "")
		if secretName != "" {
			var err error
			postgresPassword, err = kmsClient.GetSecret(secretName)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch Postgres password from KMS: %w", err)
			}
			log.Info().Msg("Loaded Postgres password from KMS")
		}
	}

	cfg.Postgres = PostgresConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnvInt("POSTGRES_PORT", 5432),
		User:     getEnv("POSTGRES_USER", "accountant"),
		Password: postgresPassword,
		Database: getEnv("POSTGRES_DB", "accountant_crm"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "require"),  // Default to require for security (Neon requires SSL)
		PoolMin:  getEnvInt("POSTGRES_POOL_MIN", 2),
		PoolMax:  getEnvInt("POSTGRES_POOL_MAX", 8),
	}

	// Redis config
	redisPassword := getEnv("REDIS_PASSWORD", "")
	if kmsClient != nil && redisPassword == "" {
		secretName := getEnv("KMS_REDIS_PASSWORD_SECRET", "")
		if secretName != "" {
			var err error
			redisPassword, err = kmsClient.GetSecret(secretName)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch Redis password from KMS: %w", err)
			}
			log.Info().Msg("Loaded Redis password from KMS")
		}
	}

	cfg.Redis = RedisConfig{
		Host:       getEnv("REDIS_HOST", "localhost"),
		Port:       getEnvInt("REDIS_PORT", 6379),
		Password:   redisPassword,
		DB:         getEnvInt("REDIS_DB", 0),
		TLSEnabled: getEnvBool("REDIS_TLS_ENABLED", false),
		PoolSize:   getEnvInt("REDIS_POOL_SIZE", 10),
	}

	// JWT config
	cfg.JWT = JWTConfig{
		SecretKey:          getEnv("JWT_SECRET_KEY", ""),
		AccessTokenExpire:  getEnvDuration("JWT_ACCESS_TOKEN_EXPIRE", 15*time.Minute),
		RefreshTokenExpire: getEnvDuration("JWT_REFRESH_TOKEN_EXPIRE", 7*24*time.Hour),
		Issuer:             getEnv("JWT_ISSUER", "accountant-crm"),
	}

	// Email config (Resend)
	cfg.Email = EmailConfig{
		APIKey:    getEnv("RESEND_API_KEY", ""),
		FromEmail: getEnv("EMAIL_FROM", "noreply@accountant-crm.com"),
		FromName:  getEnv("EMAIL_FROM_NAME", "Accountant CRM"),
	}

	// Frontend URL for email links
	cfg.FrontendURL = getEnv("FRONTEND_URL", "http://localhost:3000")

	// mTLS config
	// When SECRETS_FROM_KMS=true, fetch certs from KMS and write to temp files
	mtlsEnabled := getEnvBool("MTLS_ENABLED", false)
	var mtlsCACert, mtlsClientCert, mtlsClientKey string
	var mtlsErr error

	if mtlsEnabled && kmsClient != nil {
		// Fetch mTLS certificates from KMS
		mtlsCACert, mtlsClientCert, mtlsClientKey, mtlsErr = loadMTLSFromKMS(kmsClient)
		if mtlsErr != nil {
			return nil, fmt.Errorf("failed to load mTLS certs from KMS: %w", mtlsErr)
		}
		log.Info().Msg("Loaded mTLS certificates from KMS")
	} else {
		// Use file paths from environment (local development)
		mtlsCACert = getEnv("MTLS_CA_CERT", "")
		mtlsClientCert = getEnv("MTLS_CLIENT_CERT", "")
		mtlsClientKey = getEnv("MTLS_CLIENT_KEY", "")
	}

	cfg.MTLS = MTLSConfig{
		Enabled:    mtlsEnabled,
		CACert:     mtlsCACert,
		ServerCert: getEnv("MTLS_SERVER_CERT", ""),
		ServerKey:  getEnv("MTLS_SERVER_KEY", ""),
		ClientCert: mtlsClientCert,
		ClientKey:  mtlsClientKey,
	}

	// CORS config
	cfg.CORS = CORSConfig{
		AllowedOrigins:   getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
	}

	// Python AI service config
	cfg.PythonAI = PythonAIConfig{
		BaseURL: getEnv("PYTHON_AI_URL", "https://python-ai.fzco.local:8000"),
		Timeout: getEnvDuration("PYTHON_AI_TIMEOUT", 30*time.Second),
	}

	// Rate limiting config
	cfg.RateLimit = RateLimitConfig{
		Enabled:       getEnvBool("RATE_LIMIT_ENABLED", true),
		RequestsPerIP: getEnvInt("RATE_LIMIT_REQUESTS_PER_IP", 100),
		Window:        getEnvDuration("RATE_LIMIT_WINDOW", 1*time.Minute),
		BurstSize:     getEnvInt("RATE_LIMIT_BURST_SIZE", 20),
	}

	// Companies House API config
	cfg.CompaniesHouse = CompaniesHouseConfig{
		APIKey:   getEnv("COMPANIES_HOUSE_API_KEY", ""),
		BaseURL:  getEnv("COMPANIES_HOUSE_BASE_URL", "https://api.company-information.service.gov.uk"),
		Timeout:  getEnvDuration("COMPANIES_HOUSE_TIMEOUT", 10*time.Second),
		CacheTTL: getEnvDuration("COMPANIES_HOUSE_CACHE_TTL", 1*time.Hour),
	}

	// Validate critical security settings in production AND staging
	// Staging should mirror production security to catch issues before deployment
	if cfg.App.Env == "production" || cfg.App.Env == "staging" {
		if cfg.JWT.SecretKey == "" {
			return nil, fmt.Errorf("JWT_SECRET_KEY is required in %s", cfg.App.Env)
		}
		if len(cfg.JWT.SecretKey) < 32 {
			return nil, fmt.Errorf("JWT_SECRET_KEY must be at least 32 characters in %s", cfg.App.Env)
		}
	}

	return cfg, nil
}

// Helper functions for environment variable parsing

// getEnv returns the environment variable value or default if empty/unset.
// Use getEnvOrEmpty when you need to distinguish between empty and unset.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrEmpty returns the value if set (even if empty), otherwise the default.
// This allows explicitly setting an env var to empty string.
func getEnvOrEmpty(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// loadMTLSFromKMS fetches mTLS certificates from Alibaba Cloud KMS and writes them
// to temporary files. Returns the file paths for CA cert, client cert, and client key.
// The caller is responsible for cleaning up these files when the application exits.
func loadMTLSFromKMS(kmsClient *secrets.Client) (caCertPath, clientCertPath, clientKeyPath string, err error) {
	// Get secret names from environment
	caSecretName := getEnv("KMS_MTLS_CA_CERT_SECRET", "")
	clientCertSecretName := getEnv("KMS_MTLS_CLIENT_CERT_SECRET", "")
	clientKeySecretName := getEnv("KMS_MTLS_CLIENT_KEY_SECRET", "")

	if caSecretName == "" || clientCertSecretName == "" || clientKeySecretName == "" {
		return "", "", "", fmt.Errorf("mTLS KMS secret names not configured: CA=%q, Cert=%q, Key=%q",
			caSecretName, clientCertSecretName, clientKeySecretName)
	}

	// Fetch CA certificate
	caCert, err := kmsClient.GetSecret(caSecretName)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch CA cert from KMS: %w", err)
	}

	// Fetch client certificate
	clientCert, err := kmsClient.GetSecret(clientCertSecretName)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch client cert from KMS: %w", err)
	}

	// Fetch client key
	clientKey, err := kmsClient.GetSecret(clientKeySecretName)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch client key from KMS: %w", err)
	}

	// Write certificates to temporary files
	// Use 0600 permissions for security (owner read/write only)
	caCertPath, err = writeTempFile("mtls-ca-*.pem", caCert)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to write CA cert to temp file: %w", err)
	}

	clientCertPath, err = writeTempFile("mtls-client-cert-*.pem", clientCert)
	if err != nil {
		os.Remove(caCertPath) // Clean up on failure
		return "", "", "", fmt.Errorf("failed to write client cert to temp file: %w", err)
	}

	clientKeyPath, err = writeTempFile("mtls-client-key-*.pem", clientKey)
	if err != nil {
		os.Remove(caCertPath)     // Clean up on failure
		os.Remove(clientCertPath) // Clean up on failure
		return "", "", "", fmt.Errorf("failed to write client key to temp file: %w", err)
	}

	log.Debug().
		Str("ca_cert", caCertPath).
		Str("client_cert", clientCertPath).
		Str("client_key", clientKeyPath).
		Msg("mTLS certificates written to temp files")

	return caCertPath, clientCertPath, clientKeyPath, nil
}

// writeTempFile writes content to a temporary file with secure permissions.
// Returns the path to the created file.
// Uses os.OpenFile with explicit 0600 mode to avoid permission race condition.
func writeTempFile(pattern, content string) (string, error) {
	// Generate unique filename using CreateTemp, then recreate with secure perms
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(tmpPath) // Remove the file created with default perms

	// Recreate with explicit secure permissions (no race condition)
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}
