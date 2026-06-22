// Package main provides example
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Logging struct {
	Level  string `yaml:"level" default:"info" description:"Log level"`
	Colors bool   `yaml:"colors" description:"Colors"`
	Caller bool   `yaml:"caller" description:"Display caller information"`
	Stack  bool   `yaml:"stack" description:"Display stack information"`
}

type Server struct {
	Logging Logging `yaml:"logging"`
}

type Config struct {
	Server Server `yaml:"server"`
}

func main() {
	cfg := Config{}

	data := []byte(`
server:
  logging: !include examples/with-include/logging.yaml
`)

	err := yaml.Load(data, &cfg)
	if err != nil {
		log.Fatal("parse config: ", err.Error())
	}

	out, err := yaml.Dump(cfg)
	if err != nil {
		log.Fatal("dump config: ", err.Error())
	}
	fmt.Println(string(out))
}
