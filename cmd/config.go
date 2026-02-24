package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	db dbConfig `yaml:"db"`
}

type dbConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Name     string `yaml:"name"`
	UserName string `yaml:"username"`
	Password string `yaml:"password"`
}

func initConfig() (*config, error) {
	configRaw, err := os.ReadFile("files/config/secret.yaml")
	if err != nil {
		return nil, err
	}

	var config config
	if err := yaml.Unmarshal(configRaw, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
