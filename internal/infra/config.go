package infra

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config — корневая структура конфигурации всей платформы.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Engine   EngineConfig   `mapstructure:"engine"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

// ServerConfig описывает настройки HTTP-сервера.
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig описывает подключение к PostgreSQL.
type DatabaseConfig struct {
	URL      string `mapstructure:"url"`
	MaxConns int32  `mapstructure:"max_conns"`
	MinConns int32  `mapstructure:"min_conns"`
}

// RedisConfig описывает подключение к Redis (Pub/Sub и Cache).
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AuthConfig содержит пути к RSA ключам и настройки JWT.
type AuthConfig struct {
	PublicKeyPath  string        `mapstructure:"public_key_path"`
	PrivateKeyPath string        `mapstructure:"private_key_path"` // Только для Console API
	TokenTTL       time.Duration `mapstructure:"token_ttl"`
	BcryptCost     int           `mapstructure:"bcrypt_cost"`
	PublicKey      []byte
	PrivateKey     []byte
}

// EngineConfig содержит специфичные настройки для UAG Data Plane.
type EngineConfig struct {
	AuditBufferSize    int           `mapstructure:"audit_buffer_size"`
	AuditFlushInterval time.Duration `mapstructure:"audit_flush_interval"`

	// Настройки Circuit Breaker для внешних AI-коннекторов
	CBMaxRequests int           `mapstructure:"cb_max_requests"`
	CBInterval    time.Duration `mapstructure:"cb_interval"`
	CBTimeout     time.Duration `mapstructure:"cb_timeout"`
}

// LoggerConfig настраивает поведение zap логгера.
type LoggerConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, console
}

// LoadConfig инициализирует конфигурацию, объединяя значения из файла и ENV.
func LoadConfig() (*Config, error) {
	v := viper.New()

	// 1. Настройка поиска файла
	v.SetConfigName("config")    // имя файла без расширения
	v.SetConfigType("yaml")      // формат
	v.AddConfigPath(".")         // ищем в корне
	v.AddConfigPath("./configs") // и в папке с конфигами

	// 2. Настройка переменных окружения (ENV)
	// Позволяет перекрывать конфиг: SERVER_PORT=9000 перекроет server.port
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 3. Установка дефолтных значений
	setDefaults(v)

	// 4. Чтение файла
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Если файла нет — работаем на ENV и дефолтах
	}

	// 5. Маппинг в структуру
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	// 6. Загрузка ключей из Файла ИЛИ из ENV
	// Сначала проверяем, не лежит ли сам PEM-ключ в ENV (для Docker/K8s)
	// Если нет — читаем файл по указанному пути
	cfg.Auth.PublicKey = loadKeyResource(cfg.Auth.PublicKeyPath, "AUTH_PUBLIC_KEY_DATA")
	cfg.Auth.PrivateKey = loadKeyResource(cfg.Auth.PrivateKeyPath, "AUTH_PRIVATE_KEY_DATA")

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 5*time.Second)
	v.SetDefault("database.max_conns", 15)
	v.SetDefault("database.min_conns", 5)
	v.SetDefault("logger.level", "info")
	v.SetDefault("engine.audit_buffer_size", 1000)
	v.SetDefault("engine.audit_flush_interval", 1*time.Second)
}

// loadKeyResource — универсальный хелпер архитектора
func loadKeyResource(path string, envDataKey string) []byte {
	// Если ключ прилетел напрямую в ENV (Base64 или PEM)
	if data := os.Getenv(envDataKey); data != "" {
		return []byte(data)
	}
	// Иначе читаем файл по пути из конфига
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return data
		}
	}
	return nil
}
