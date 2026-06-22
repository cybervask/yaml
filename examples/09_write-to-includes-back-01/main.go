// Package main provides example
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cybervask/yaml"
)

type Logging struct {
	Level  string `yaml:"level" default:"info" description:"Log level"`
	Colors bool   `yaml:"colors" description:"Colors"`
	Caller bool   `yaml:"caller" description:"Display caller information"`
	Stack  bool   `yaml:"stack" description:"Display stack information"`
}

type Config struct {
	Server struct {
		// Logging Logging `yaml:"logging" include:"./examples/write-to-includes/logging.yaml"`
		Logging Logging `yaml:"logging"`
	} `yaml:"server"`
}

func main() {
	var cfg Config

	// 1. Загрузка с треккингом путей
	yaml.ResetIncludeTracker()
	includeFile := "./examples/write-to-includes/config.yaml"
	if err := yaml.UnmarshalFile(includeFile, &cfg); err != nil {
		log.Fatal(err)
	}

	// 2. Проверка пути инклуда
	if p := yaml.FindIncludeFile("server.logging"); p != "" {
		fmt.Printf("✅ server.logging: include path: %s\n", p)
	} else {
		fmt.Println("ℹ️ Logging is inline (not from !include)")
	}

	cfg.Server.Logging.Level = "debug"

	// 3. Атомарный дамп с сохранением структуры инклудов
	out, err := yaml.DumpWithInclude(&cfg, yaml.WithRelativeIncludes("."))
	if err != nil {
		log.Fatal(err)
	}

	// out содержит YAML с !include, а изменённые данные записаны в файлы атомарно
	_ = os.WriteFile("./examples/write-to-includes/config.yaml", out, 0644)
}
