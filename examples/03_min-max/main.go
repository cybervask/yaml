// Package main provides example
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Compression struct {
	MinSize int `yaml:"min_size" validate:"min=4"`
	Level   int `yaml:"level" validate:"min=0"`
}

type Config struct {
	Compression Compression `yaml:"compression"`
}

func main() {
	cfg := Config{}
	data := []byte(`
compression:
  min_size: 5
  level: 0
`)

	err := yaml.Load(data, &cfg)
	if err != nil {
		log.Fatal("parse config:", err.Error())
	}

	out, err := yaml.Dump(cfg)
	if err != nil {
		log.Fatal("dump config:", err.Error())
	}

	fmt.Println(string(out))
}
