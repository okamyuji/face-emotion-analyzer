package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoader_LoadConfig(t *testing.T) {
	// テスト前に環境をクリーン
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)
	os.Unsetenv("APP_ENV")

	tests := []struct {
		name     string
		setup    func(dir string) error
		env      map[string]string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "基本設定の読み込み",
			setup: func(dir string) error {
				baseConfig := `
app:
  name: "face-emotion-analyzer"
  env: "development"
  debug: true
server:
  port: "8080"
  host: "localhost"
security:
  csrf_token_length: 32
  rate_limit:
    requests_per_minute: 100
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
				devConfig := `
app:
  debug: true
server:
  host: "localhost"
`
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(baseConfig), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "config.development.yaml"), []byte(devConfig), 0644)
			},
			validate: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg)
				assert.Equal(t, "face-emotion-analyzer", cfg.App.Name)
				assert.Equal(t, "8080", cfg.Server.Port)
				assert.Equal(t, "development", cfg.App.Env)
				assert.True(t, cfg.App.Debug)
			},
		},
		{
			name: "環境変数による上書き",
			setup: func(dir string) error {
				config := `
app:
  name: "face-emotion-analyzer"
  env: "development"
server:
  port: "8080"
security:
  csrf_token_length: 32
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0644); err != nil {
					return err
				}

				prodConfig := `
app:
  env: "production"
server:
  port: "80"
image:
  max_size: 5242880
`
				if err := os.WriteFile(filepath.Join(dir, "config.production.yaml"), []byte(prodConfig), 0644); err != nil {
					return err
				}

				testConfig := `
app:
  env: "test"
server:
  port: "8081"
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
				return os.WriteFile(filepath.Join(dir, "config.test.yaml"), []byte(testConfig), 0644)
			},
			env: map[string]string{
				"APP_ENV": "production",
				"PORT":    "3000",
			},
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "production", cfg.App.Env)
				assert.Equal(t, "3000", cfg.Server.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テスト用の一時ディレクトリを作成
			dir := t.TempDir()

			// 設定ファイルの作成
			if tt.setup != nil {
				err := tt.setup(dir)
				require.NoError(t, err)
			}

			// 環境変数の設定
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			// ConfigLoaderの作成とテスト
			loader := NewConfigLoader(dir)
			cfg, err := loader.LoadConfig()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestConfigLoader_WatchConfig(t *testing.T) {
	// テスト前に環境をクリーン
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)
	os.Unsetenv("APP_ENV")

	dir := t.TempDir()

	initialConfig := `
app:
  name: "face-emotion-analyzer"
  env: "development"
server:
  port: "8080"
security:
  csrf_token_length: 32
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
	devConfig := `
app:
  debug: true
server:
  host: "localhost"
`
	testConfig := `
app:
  env: "test"
server:
  port: "8081"
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "config.development.yaml"), []byte(devConfig), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "config.test.yaml"), []byte(testConfig), 0644)
	require.NoError(t, err)

	loader := NewConfigLoader(dir)
	configChanged := make(chan *Config, 1)

	err = loader.WatchConfig(func(cfg *Config) {
		configChanged <- cfg
	})
	require.NoError(t, err)

	updatedConfig := `
app:
  name: "face-emotion-analyzer"
  env: "development"
server:
  port: "3000"
security:
  csrf_token_length: 32
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`
	time.Sleep(100 * time.Millisecond)
	err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	select {
	case cfg := <-configChanged:
		assert.Equal(t, "3000", cfg.Server.Port)
	case <-time.After(2 * time.Second):
		t.Fatal("設定の変更が検知されませんでした")
	}
}

func TestConfigLoader_ValidateConfig(t *testing.T) {
	// テスト前に環境をクリーン
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)
	os.Unsetenv("APP_ENV")

	tests := []struct {
		name       string
		config     string
		devConfig  string
		testConfig string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "有効な設定",
			config: `
app:
  name: "face-emotion-analyzer"
  env: "development"
server:
  port: "8080"
security:
  csrf_token_length: 32
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`,
			devConfig: `
app:
  debug: true
`,
			testConfig: `
app:
  env: "test"
server:
  port: "8081"
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`,
			wantErr: false,
		},
		{
			name: "アプリケーション名なし",
			config: `
app:
  env: "development"
server:
  port: "8080"
security:
  csrf_token_length: 32
image:
  max_size: 5242880
`,
			devConfig: `
app:
  debug: true
`,
			testConfig: `
app:
  env: "test"
server:
  port: "8081"
image:
  max_size: 5242880
opencv:
  scale_factor: 1.1
`,
			wantErr: true,
			errMsg:  "設定の検証に失敗: アプリケーション名が設定されていません",
		},
		{
			name: "不正なCSRFトークン長",
			config: `
app:
  name: "face-emotion-analyzer"
  env: "development"
  debug: false
server:
  port: "8080"
  host: "localhost"
security:
  csrf_token_length: 16
  rate_limit:
    requests_per_minute: 100
image:
  max_size: 5242880
opencv:
  scale_factor: 1.2
logging:
  level: "info"
  format: "json"
`,
			devConfig: `
app:
  debug: true
server:
  host: "localhost"
opencv:
  scale_factor: 1.2
logging:
  level: "debug"
`,
			testConfig: `
app:
  env: "test"
server:
  port: "8081"
image:
  max_size: 5242880
opencv:
  scale_factor: 1.2
logging:
  level: "debug"
`,
			wantErr: true,
			errMsg:  "設定の検証に失敗: CSRFトークンの長さは32以上である必要があります",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.config), 0644)
			require.NoError(t, err)

			devConfigPath := filepath.Join(dir, "config.development.yaml")
			err = os.WriteFile(devConfigPath, []byte(tt.devConfig), 0644)
			require.NoError(t, err)

			testConfigPath := filepath.Join(dir, "config.test.yaml")
			err = os.WriteFile(testConfigPath, []byte(tt.testConfig), 0644)
			require.NoError(t, err)

			loader := NewConfigLoader(dir)
			err = loader.ValidateConfig()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
