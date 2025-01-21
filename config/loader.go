package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// 設定ファイルの読み込み
type ConfigLoader struct {
	configDir string
}

// 新しいConfigLoaderを作成
func NewConfigLoader(configDir string) *ConfigLoader {
	return &ConfigLoader{
		configDir: configDir,
	}
}

// 環境に応じた設定ファイルを読み込む
func (l *ConfigLoader) LoadConfig() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// 基本設定の読み込み
	baseConfig, err := l.loadConfigFile("config.yaml")
	if err != nil {
		return nil, err
	}

	// 環境固有の設定の読み込み
	envConfig, err := l.loadConfigFile(fmt.Sprintf("config.%s.yaml", env))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 設定のマージ
	if err := l.mergeConfigs(baseConfig, envConfig); err != nil {
		return nil, err
	}

	// 環境変数による上書き
	if err := l.overrideWithEnv(baseConfig); err != nil {
		return nil, err
	}

	// 設定の検証
	if err := baseConfig.Validate(); err != nil {
		return nil, fmt.Errorf("設定の検証に失敗: %w", err)
	}

	return baseConfig, nil
}

// 指定された設定ファイルを読み込む
func (l *ConfigLoader) loadConfigFile(filename string) (*Config, error) {
	data, err := l.loadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("YAMLのパースに失敗: %w", err)
	}

	return &config, nil
}

// 指定されたファイルを読み込む
func (l *ConfigLoader) loadFile(filename string) ([]byte, error) {
	// パスのバリデーション
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		return nil, fmt.Errorf("不正なファイル形式です: %s", filename)
	}

	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("不正なファイルパスです: %s", filename)
	}

	filepath := filepath.Join(l.configDir, cleanPath)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
	}
	return data, nil
}

// 2つの設定をマージする
func (l *ConfigLoader) mergeConfigs(base, override *Config) error {
	if override == nil {
		return nil
	}

	// 環境設定の上書き
	if override.App.Env != "" {
		base.App.Env = override.App.Env
	}
	if override.App.Debug {
		base.App.Debug = true
	}

	// サーバー設定の上書き
	if override.Server.Port != "" {
		base.Server.Port = override.Server.Port
	}

	return nil
}

// 環境変数で設定を上書きする
func (l *ConfigLoader) overrideWithEnv(config *Config) error {
	// アプリケーション設定
	if env := os.Getenv("APP_ENV"); env != "" {
		config.App.Env = env
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		config.App.Debug = debug == "true"
	}

	// サーバー設定
	if port := os.Getenv("PORT"); port != "" {
		config.Server.Port = port
	}
	if host := os.Getenv("HOST"); host != "" {
		config.Server.Host = host
	}

	// セキュリティ設定
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		config.Security.AllowedOrigins = origins
	}
	if rateLimit := os.Getenv("RATE_LIMIT_REQUESTS"); rateLimit != "" {
		limit, err := strconv.Atoi(rateLimit)
		if err != nil {
			return fmt.Errorf("RATE_LIMIT_REQUESTSの解析に失敗: %w", err)
		}
		config.Security.RateLimit.RequestsPerMinute = limit
	}

	// ロギング設定
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		config.Logging.Format = logFormat
	}

	return nil
}

// 指定されたパスの設定値を取得
func (l *ConfigLoader) GetConfigValue(path string) (interface{}, error) {
	config, err := l.LoadConfig()
	if err != nil {
		return nil, err
	}

	parts := strings.Split(path, ".")
	current := interface{}(config)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("設定が見つかりません: %s", path)
			}
		default:
			return nil, fmt.Errorf("無効な設定パス: %s", path)
		}
	}

	return current, nil
}

// 指定されたパスに設定値を設定
func (l *ConfigLoader) SetConfigValue(path string, value interface{}) error {
	config, err := l.LoadConfig()
	if err != nil {
		return err
	}

	parts := strings.Split(path, ".")
	current := interface{}(config)

	for _, part := range parts[:len(parts)-1] {
		switch v := current.(type) {
		case map[string]interface{}:
			next, ok := v[part]
			if !ok {
				v[part] = make(map[string]interface{})
				next = v[part]
			}
			current = next
		default:
			return fmt.Errorf("無効な設定パス: %s", path)
		}
	}

	// 変更を設定ファイルに保存
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("YAML形式への変換に失敗: %w", err)
	}

	filename := filepath.Join(l.configDir, "config.yaml")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("設定ファイルの保存に失敗: %w", err)
	}

	return nil
}

// 設定の検証
func (l *ConfigLoader) ValidateConfig() error {
	cfg, err := l.LoadConfig()
	if err != nil {
		return err
	}

	if cfg.App.Name == "" {
		return fmt.Errorf("設定の検証に失敗: アプリケーション名が設定されていません")
	}

	if cfg.Server.Port == "" {
		return fmt.Errorf("設定の検証に失敗: サーバーポートが設定されていません")
	}

	if cfg.Security.CSRFTokenLength < 32 {
		return fmt.Errorf("設定の検証に失敗: CSRFトークンの長さは32以上である必要があります")
	}

	if cfg.Image.MaxSize <= 0 {
		return fmt.Errorf("設定の検証に失敗: 不正な最大画像サイズです")
	}

	if cfg.OpenCV.ScaleFactor <= 1.0 {
		return fmt.Errorf("設定の検証に失敗: 不正なスケールファクターです")
	}

	return nil
}

// 設定ファイルの変更を監視
func (l *ConfigLoader) WatchConfig(callback func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("設定の監視開始に失敗: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// 設定ファイルが変更された場合
					cfg, err := l.LoadConfig()
					if err != nil {
						log.Printf("設定の再読み込みに失敗: %v", err)
						continue
					}
					callback(cfg)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("設定の監視中にエラー: %v", err)
			}
		}
	}()

	// 設定ファイルを監視対象に追加
	configFile := filepath.Join(l.configDir, "config.yaml")
	if err := watcher.Add(configFile); err != nil {
		watcher.Close()
		return fmt.Errorf("設定ファイルの監視に失敗: %w", err)
	}

	return nil
}

// 現在の環境を取得
func (l *ConfigLoader) GetEnvironment() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	return env
}

// 本番環境かどうかを判定
func (l *ConfigLoader) IsProduction() bool {
	return l.GetEnvironment() == "production"
}

// 開発環境かどうかを判定
func (l *ConfigLoader) IsDevelopment() bool {
	return l.GetEnvironment() == "development"
}

// テスト環境かどうかを判定
func (l *ConfigLoader) IsTest() bool {
	return l.GetEnvironment() == "test"
}

// テンプレート設定を読み込む
func (l *ConfigLoader) LoadTemplate(templateName string) (*Config, error) {
	filename := filepath.Join(l.configDir, "templates", templateName+".yaml")
	return l.loadConfigFile(filepath.Base(filename))
}

// 現在の設定をテンプレートとして保存
func (l *ConfigLoader) SaveAsTemplate(templateName string) error {
	config, err := l.LoadConfig()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	templatesDir := filepath.Join(l.configDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(templatesDir, templateName+".yaml")
	return os.WriteFile(filename, data, 0644)
}
