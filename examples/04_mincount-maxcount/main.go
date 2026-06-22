// Package main provides example
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Quantities struct {
	Tickets         []string          `yaml:"tickets" validate:"mincount=3"`
	PersonalTickets map[string]string `yaml:"personal_tickets" validate:"maxcount=2"`
}

type Config struct {
	Quantities Quantities `yaml:"quantities"`
}

func main() {
	cfg := Config{}
	data := []byte(`
quantities:
  tickets:
    - ticket1
    - ticket2
    - ticket3
  personal_tickets:
    alice: ticket1
    helen: ticket2
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
