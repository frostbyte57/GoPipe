package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DownloadDir string `json:"download_dir"`
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".gopipe")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func LoadConfig() (*Config, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		// Return default
		home, _ := os.UserHomeDir()
		return &Config{DownloadDir: home}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(cfg)
}
