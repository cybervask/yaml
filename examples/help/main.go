// Package main provides example
package main

import (
	"github.com/cybervask/yaml"
)

type Compression struct {
	MinSize int `yaml:"min_size" validate:"min=4" description:"Minimum size of file"`
	Level   int `yaml:"level" validate:"min=0" description:"Minimum compression level"`
}

type Config struct {
	Compression Compression `yaml:"compression" description:"Apply compression to data"`
}

func main() {
	cfg := Config{}
	yaml.Help(cfg)
}
