// Package main demonstrates automated CLI help generation via yaml.Help(),
// which renders a configuration schema from struct tags.
package main

import (
	"time"

	"github.com/cybervask/yaml"
)

// Compression describes archive processing tuning knobs.
type Compression struct {
	MinSize int `yaml:"min_size" validate:"min=4" description:"Minimum size of file"`
	Level   int `yaml:"level" validate:"min=0,max=9" description:"Compression level (inclusive bounds)"`

	Interval time.Duration `yaml:"interval" validate:"min=1s,max=10m" description:"Compaction interval"`
}

// Config is the root configuration model for the application.
type Config struct {
	Compression Compression `yaml:"compression" description:"Apply compression to data"`
}

func main() {
	// Help prints the schema documentation built from yaml/validate/description tags.
	cfg := Config{}
	yaml.Help(cfg)
}
