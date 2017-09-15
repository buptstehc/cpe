package util

import (
	"os"
	"encoding/json"
	"fmt"
)

type Config struct {
	IP           []string
	Qps          int
	Cpe          int
	ClientsPerIP int
	Vertx        string
	Mac          string
	SN           string
	LOID         string
	MetricPort   int
}

func ReadConfig(filePath string) (Config, bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("open config file meets error:", err)
		return Config{}, false
	}

	decoder := json.NewDecoder(file)
	config := Config{Qps: 1, Cpe: 5}
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println("parse config file meets error:", err)
		return Config{}, false
	}

	return config, true
}
