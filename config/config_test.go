package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	content := []byte(`
ds_api_key: "test-key"
ds_base_url: "http://test.com"
ds_model: "test-model"
server_port: "8080"
prompts:
  translate: "test translate %s"
  format: "test format %s"
  summarize: "test summarize %s"
`)
	
	tmpfile, err := os.CreateTemp("", "config.*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	
	cfg := Load()
	
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"DSApiKey", cfg.DSApiKey, "test-key"},
		{"DSBaseURL", cfg.DSBaseURL, "http://test.com"},
		{"DSModel", cfg.DSModel, "test-model"},
		{"ServerPort", cfg.ServerPort, "8080"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %v, want %v", tt.got, tt.expected)
			}
		})
	}
}