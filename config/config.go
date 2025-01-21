package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// アプリケーション設定
type Config struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Env     string `yaml:"env"`
		Debug   bool   `yaml:"debug"`
	} `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Security SecurityConfig `yaml:"security"`
	Image    ImageConfig    `yaml:"image"`
	OpenCV   OpenCVConfig   `yaml:"opencv"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// サーバー設定
type ServerConfig struct {
	Port           string        `yaml:"port"`
	Host           string        `yaml:"host"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
}

// セキュリティ設定
type SecurityConfig struct {
	AllowedOrigins  string            `yaml:"allowed_origins"`
	CSRFTokenLength int               `yaml:"csrf_token_length"`
	RateLimit       RateLimitConfig   `yaml:"rate_limit"`
	CORS            CORSConfig        `yaml:"cors"`
	Headers         map[string]string `yaml:"headers"`
}

// レートリミット設定
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst             int `yaml:"burst"`
}

// CORS設定
type CORSConfig struct {
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

// 画像設定
type ImageConfig struct {
	MaxSize      int64    `yaml:"max_size"`
	AllowedTypes []string `yaml:"allowed_types"`
	MaxDimension int      `yaml:"max_dimension"`
	Quality      int      `yaml:"quality"`
}

// OpenCV設定
type OpenCVConfig struct {
	CascadeFile  string  `yaml:"cascade_file"`
	MinFaceSize  int     `yaml:"min_face_size"`
	ScaleFactor  float64 `yaml:"scale_factor"`
	MinNeighbors int     `yaml:"min_neighbors"`
	Flags        int     `yaml:"flags"`
}

// ログ設定
type LoggingConfig struct {
	Level  string            `yaml:"level"`
	Format string            `yaml:"format"`
	Output string            `yaml:"output"`
	Fields map[string]string `yaml:"fields"`
}

// 設定ファイルを読み込む
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みエラー: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("設定ファイルのパースエラー: %w", err)
	}

	// 環境変数からの上書き
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.App.Env = env
	}
	if debug := os.Getenv("DEBUG"); debug == "true" {
		cfg.App.Debug = true
	}
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = port
	}

	return &cfg, nil
}

// 設定値の検証
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("アプリケーション名が設定されていません")
	}
	if c.Server.Port == "" {
		return fmt.Errorf("サーバーポートが設定されていません")
	}
	if c.Image.MaxSize <= 0 {
		return fmt.Errorf("不正な最大画像サイズです")
	}
	if c.OpenCV.ScaleFactor <= 1.0 {
		return fmt.Errorf("不正なスケールファクターです")
	}
	return nil
}

// 開発環境かどうかを判定
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// 本番環境かどうかを判定
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
