// Package main demonstrates the min/max inclusive boundary validation rules
// for integers and time.Duration values.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybervask/yaml"
)

// Compression describes archive processing tuning knobs.
type Compression struct {
	// MinSize must be greater than or equal to 4.
	MinSize int `yaml:"min_size" validate:"min=4" description:"Minimum size of file"`
	// Level must stay within the inclusive 0..9 range.
	Level int `yaml:"level" validate:"min=0,max=9" description:"Compression level (inclusive bounds)"`
	// Interval must be between 1s and 10m inclusive.
	Interval time.Duration `yaml:"interval" validate:"min=1s,max=10m" description:"Compaction interval"`
}

// Config is the root configuration model for the application.
type Config struct {
	Compression Compression `yaml:"compression" description:"Compaction settings"`
}

func main() {
	// Valid profile: every value lies inside its inclusive bounds.
	valid := Config{}
	if err := yaml.Load([]byte(`
compression:
  min_size: 5
  level: 6
  interval: 30s
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: level 10 exceeds max=9 and interval 20m exceeds max=10m.
	invalid := Config{}
	err = yaml.Load([]byte(`
compression:
  min_size: 5
  level: 10
  interval: 20m
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("out-of-range profile rejected as expected:")
	fmt.Println(err)
}
