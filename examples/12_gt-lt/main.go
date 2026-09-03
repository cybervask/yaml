// Package main demonstrates the gt/lt strict boundary validation rules.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybervask/yaml"
)

type Balancer struct {
	// Weight must be strictly greater than 0 and strictly less than 100.
	Weight int `yaml:"weight" validate:"gt=0,lt=100" description:"Balancer node weight"`
	// Timeout must be strictly greater than 100ms and strictly less than 10s.
	Timeout time.Duration `yaml:"timeout" validate:"gt=100ms,lt=10s" description:"Request timeout"`
	// Ratio must be strictly greater than 0 and strictly less than 1.
	Ratio float64 `yaml:"ratio" validate:"gt=0,lt=1" description:"Traffic split ratio"`
}

type Config struct {
	Balancer Balancer `yaml:"balancer"`
}

func main() {
	// Valid profile: all values are inside the strict (gt, lt) boundaries.
	valid := Config{}
	if err := yaml.Load([]byte(`
balancer:
  weight: 50
  timeout: 5s
  ratio: 0.5
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Boundary violation: weight=100 fails lt=100, timeout=100ms fails gt=100ms, ratio=1 fails lt=1.
	invalid := Config{}
	err = yaml.Load([]byte(`
balancer:
  weight: 100
  timeout: 100ms
  ratio: 1
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("boundary values rejected as expected (gt/lt are strict):")
	fmt.Println(err)
}
