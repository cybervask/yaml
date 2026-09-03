// Package main demonstrates per-instance path rules: a shared nested type
// whose fields get different validation bounds depending on the parent field,
// expressed via <yaml_path>.<rule>=<value> tokens in the validate tag.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// ServerDesc is shared between two server description fields; every bound is
// defined per instance on the parent field, not here.
type ServerDesc struct {
	Description string `yaml:"description" description:"Free-form description text"`
	MinLen      int    `yaml:"min_len" description:"Minimum length boundary"`
	MaxLen      int    `yaml:"max_len" description:"Maximum length boundary"`
}

// Server describes the listening endpoint and its worker pool.
type Server struct {
	BindEndpoint string `yaml:"bind_endpoint" validate:"endpoint" description:"host:port bind address"`
	Workers      int    `yaml:"workers" validate:"min=1,max=64" description:"Worker pool size"`

	// Path rules give each shared-type instance its own bounds:
	// server_desc1 allows min_len >= 0 and max_len <= 64,
	// server_desc2 requires min_len >= 4 and max_len <= 32.
	ServerDesc1 ServerDesc `yaml:"server_desc1" validate:"min_len.min=0,max_len.max=64" description:"Server Description 1"`
	ServerDesc2 ServerDesc `yaml:"server_desc2" validate:"min_len.min=4,max_len.max=32" description:"Server Description 2"`
}

// Config is the root configuration model for the application.
type Config struct {
	Server     Server `yaml:"server" description:"Server settings"`
	AdminEmail string `yaml:"admin_email" validate:"regexp=^[^@]+@[^@]+$" description:"Administrator contact"`
}

func main() {
	// Valid profile: every instance satisfies its own path-rule bounds.
	valid := Config{}
	if err := yaml.Load([]byte(`
server:
  bind_endpoint: "127.0.0.1:443"
  workers: 8
  server_desc1:
    description: some description 1
    min_len: 0
    max_len: 10
  server_desc2:
    description: some description 2
    min_len: 5
    max_len: 20
admin_email: "admin@cybervask.net"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: min_len/max_len break the per-instance bounds
	// (server_desc1: min 0 / max 64, server_desc2: min 4 / max 32).
	invalid := Config{}
	err = yaml.Load([]byte(`
server:
  bind_endpoint: "127.0.0.1:443"
  workers: 8
  server_desc1:
    description: some description 1
    min_len: -1
    max_len: 100
  server_desc2:
    description: some description 2
    min_len: 2
    max_len: 50
admin_email: "admin@cybervask.net"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("per-instance violations rejected as expected:")
	fmt.Println(err)
}
