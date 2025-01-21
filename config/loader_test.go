package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigLoader_LoadConfig(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir := t.TempDir()

	// テスト用の設定ファイルを作成
	baseConfig := `
app:
  name: test-app
  version: 1.0.0
  env: development
server:
  port: "8080"
  host: localhost
image:
  max_size: 10485760
  max_dimension: 1920
  quality: 90
  allowed_types:
    - image/jpeg
opencv:
  min_face_size: 30
  scale_factor: 1.1
  min_neighbors: 3
security:
  csrf_token_length: 32
  rate_limit:
    requests_per_minute: 100
    burst: 50
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(baseConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// 環境固有の設定ファイルを作成
	devConfig := `
app:
  env: development
  debug: true
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.development.yaml"), []byte(devConfig), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tempDir)

	tests := []struct {
		name    string
		envVars map[string]string
		want    *Config
		wantErr bool
	}{
		{
			name: "基本設定の読み込み",
			envVars: map[string]string{
				"APP_ENV": "development",
			},
			want: &Config{
				App: struct {
					Name    string `yaml:"name"`
					Version string `yaml:"version"`
					Env     string `yaml:"env"`
					Debug   bool   `yaml:"debug"`
				}{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
					Debug:   true,
				},
				Server: ServerConfig{
					Port:           "8080",
					Host:           "localhost",
					ReadTimeout:    5 * time.Second,
					WriteTimeout:   30 * time.Second,
					IdleTimeout:    120 * time.Second,
					MaxHeaderBytes: 1048576,
				},
				Image: ImageConfig{
					MaxSize:      10485760,
					MaxDimension: 1920,
					Quality:      90,
					AllowedTypes: []string{"image/jpeg"},
				},
				OpenCV: OpenCVConfig{
					MinFaceSize:  30,
					ScaleFactor:  1.1,
					MinNeighbors: 3,
				},
				Security: SecurityConfig{
					CSRFTokenLength: 32,
					RateLimit: RateLimitConfig{
						RequestsPerMinute: 100,
						Burst:             50,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "環境変数による上書き",
			envVars: map[string]string{
				"APP_ENV": "production",
				"PORT":    "9090",
			},
			want: &Config{
				App: struct {
					Name    string `yaml:"name"`
					Version string `yaml:"version"`
					Env     string `yaml:"env"`
					Debug   bool   `yaml:"debug"`
				}{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "production",
					Debug:   false,
				},
				Server: ServerConfig{
					Port:           "9090",
					Host:           "localhost",
					ReadTimeout:    5 * time.Second,
					WriteTimeout:   30 * time.Second,
					IdleTimeout:    120 * time.Second,
					MaxHeaderBytes: 1048576,
				},
				Image: ImageConfig{
					MaxSize:      10485760,
					MaxDimension: 1920,
					Quality:      90,
					AllowedTypes: []string{"image/jpeg"},
				},
				OpenCV: OpenCVConfig{
					MinFaceSize:  30,
					ScaleFactor:  1.1,
					MinNeighbors: 3,
				},
				Security: SecurityConfig{
					CSRFTokenLength: 32,
					RateLimit: RateLimitConfig{
						RequestsPerMinute: 100,
						Burst:             50,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 環境変数を設定
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			got, err := loader.LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// 基本的なフィールドの比較
				if got.App.Name != tt.want.App.Name {
					t.Errorf("App.Name = %v, want %v", got.App.Name, tt.want.App.Name)
				}
				if got.App.Version != tt.want.App.Version {
					t.Errorf("App.Version = %v, want %v", got.App.Version, tt.want.App.Version)
				}
				if got.Server.Port != tt.want.Server.Port {
					t.Errorf("Server.Port = %v, want %v", got.Server.Port, tt.want.Server.Port)
				}
				if got.Image.MaxSize != tt.want.Image.MaxSize {
					t.Errorf("Image.MaxSize = %v, want %v", got.Image.MaxSize, tt.want.Image.MaxSize)
				}
			}
		})
	}
}

func TestConfigLoader_WatchConfig(t *testing.T) {
	tempDir := t.TempDir()

	// 初期設定ファイルを作成
	initialConfig := `
app:
  name: test-app
  version: 1.0.0
server:
  port: "8080"
image:
  max_size: 10485760
  max_dimension: 1920
opencv:
  scale_factor: 1.1
`
	configFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(initialConfig), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewConfigLoader(tempDir)

	// 設定変更を検知するチャネル
	changes := make(chan *Config, 1)

	// 監視を開始
	err := loader.WatchConfig(func(cfg *Config) {
		changes <- cfg
	})
	if err != nil {
		t.Fatal(err)
	}

	// 設定ファイルを更新
	updatedConfig := `
app:
  name: test-app-updated
  version: 1.0.1
server:
  port: "8080"
image:
  max_size: 10485760
  max_dimension: 1920
opencv:
  scale_factor: 1.1
`
	time.Sleep(100 * time.Millisecond) // ファイルシステムの更新を確実にするため
	if err := os.WriteFile(configFile, []byte(updatedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// 変更の検知を待機
	select {
	case cfg := <-changes:
		if cfg.App.Name != "test-app-updated" {
			t.Errorf("App.Name = %v, want test-app-updated", cfg.App.Name)
		}
		if cfg.App.Version != "1.0.1" {
			t.Errorf("App.Version = %v, want 1.0.1", cfg.App.Version)
		}
	case <-time.After(2 * time.Second):
		t.Error("設定の変更が検知されませんでした")
	}
}

func TestConfigLoader_ValidateConfig(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		config     string
		wantErr    bool
		errMessage string
	}{
		{
			name: "有効な設定",
			config: `
app:
  name: test-app
  version: 1.0.0
server:
  port: "8080"
security:
  csrf_token_length: 32
  rate_limit:
    requests_per_minute: 100
image:
  max_size: 10485760
  max_dimension: 1920
  allowed_types:
    - image/jpeg
opencv:
  min_face_size: 30
  scale_factor: 1.1
`,
			wantErr: false,
		},
		{
			name: "アプリケーション名なし",
			config: `
app:
  version: 1.0.0
server:
  port: "8080"
image:
  max_size: 10485760
opencv:
  scale_factor: 1.1
`,
			wantErr:    true,
			errMessage: "設定の検証に失敗: アプリケーション名が設定されていません",
		},
		{
			name: "不正なCSRFトークン長",
			config: `
app:
  name: test-app
  version: 1.0.0
server:
  port: "8080"
security:
  csrf_token_length: 16
image:
  max_size: 10485760
opencv:
  scale_factor: 1.1
`,
			wantErr:    true,
			errMessage: "設定の検証に失敗: CSRFトークンの長さは32以上である必要があります",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tempDir, "config.yaml")
			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatal(err)
			}

			loader := NewConfigLoader(tempDir)
			err := loader.ValidateConfig()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && err.Error() != tt.errMessage {
				t.Errorf("ValidateConfig() error message = %v, want %v", err.Error(), tt.errMessage)
			}
		})
	}
}
