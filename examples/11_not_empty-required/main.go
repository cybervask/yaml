// Package main demonstrates the required validation rule and its legacy not_empty alias.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Config struct {
	// DSN must be present in YAML (or injected via env): empty value fails validation.
	// 'required' is the canonical rule name.
	DSN string `yaml:"dsn" validate:"required" description:"Database connection string"`
	// Token uses the legacy 'not_empty' alias, which behaves identically to required.
	// not_empty is planned for deprecation but kept for backward compatibility.
	Token string `yaml:"token" validate:"not_empty" description:"API auth token (legacy alias example)"`
	// Secret is optional: no validation rule, empty value is allowed.
	Secret string `yaml:"secret" description:"Optional additional secret"`
}

func main() {
	// Valid profile: both required fields are provided.
	valid := Config{}
	if err := yaml.Load([]byte(`
dsn: "postgres://user@localhost:5432/app"
token: "abc123"
secret: ""
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Invalid profile: dsn and token are empty -> two aggregated validation errors.
	invalid := Config{}
	err = yaml.Load([]byte(`
dsn: ""
token: ""
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("invalid profile rejected as expected:")
	fmt.Println(err)
}
