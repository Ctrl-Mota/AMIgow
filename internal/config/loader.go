package config

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

func Load() (*Config, error) {
	return loadFromFile("config.json")
}

func loadFromAPI(apiURL string) (*Config, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func loadFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
