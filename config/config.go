package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	Params   string `mapstructure:"params"` // extra DSN params, e.g. "parseTime=true&loc=Local"
}

type JWTConfig struct {
	Secret             string `mapstructure:"secret"`
	ExpireHours        int    `mapstructure:"expire_hours"`
	RefreshExpireHours int    `mapstructure:"refresh_expire_hours"`
}

// EncryptionAppCredential 定义一个允许接入的客户端应用及其对称密钥。
type EncryptionAppCredential struct {
	ID     string `mapstructure:"id"`
	Secret string `mapstructure:"secret"`
}

// EncryptionConfig 控制 APP 接口的加密/签名中间件。
// 启用后,/api/app/v1/* 需按约定携带 X-App-Id / X-Timestamp / X-Nonce / X-Sign,
// 且 body 需为 { "data": base64(iv||ciphertext||tag) } 的 AES-256-GCM 密文。
type EncryptionConfig struct {
	Enabled              bool                      `mapstructure:"enabled"`
	TimestampSkewSeconds int                       `mapstructure:"timestamp_skew_seconds"`
	NonceTTLSeconds      int                       `mapstructure:"nonce_ttl_seconds"`
	Apps                 []EncryptionAppCredential `mapstructure:"apps"`
}

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
}

// Load reads config.yaml (searched in ./config and .) and applies env overrides.
// Env keys use the form SECTION_KEY, e.g. JWT_SECRET, APP_PORT, DB_HOST.
func Load() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("app.port", "APP_PORT")
	_ = v.BindEnv("app.mode", "GIN_MODE")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.name", "DB_NAME")
	_ = v.BindEnv("database.params", "DB_PARAMS")
	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("jwt.expire_hours", "JWT_EXPIRE_HOURS")
	_ = v.BindEnv("jwt.refresh_expire_hours", "JWT_REFRESH_EXPIRE_HOURS")
	_ = v.BindEnv("encryption.enabled", "ENCRYPTION_ENABLED")
	_ = v.BindEnv("encryption.timestamp_skew_seconds", "ENCRYPTION_TIMESTAMP_SKEW_SECONDS")
	_ = v.BindEnv("encryption.nonce_ttl_seconds", "ENCRYPTION_NONCE_TTL_SECONDS")

	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	return &cfg
}
