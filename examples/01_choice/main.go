// Package main provides example
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type ACL struct {
	Order  string `yaml:"order" validate:"choice='allow,deny','deny,allow'"`
	Order1 string `yaml:"order1" validate:"choice=allow,deny"`
}

func main() {
	cfg := ACL{}
	data := []byte(`
order: allow,deny
order1: allow
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
