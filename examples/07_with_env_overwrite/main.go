// Package main demonstrates environment variable injection: values provided
// via the env tag override both YAML fields and tag defaults.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cybervask/yaml"
)

// TLS describes transport security version constraints.
type TLS struct {
	MinVersion string `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
	MaxVersion string `yaml:"max_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MAX_VERSION" description:"Maximum TLS version"`
}

// Client groups outgoing connection settings.
type Client struct {
	TLS TLS `yaml:"tls" description:"Client TLS configuration"`
}

// Server groups incoming listener settings.
type Server struct {
	TLS TLS `yaml:"tls" description:"Server TLS configuration"`
}

// Config is the root configuration model for the application.
type Config struct {
	Client Client `yaml:"client" description:"Client configuration"`
	Server Server `yaml:"server" description:"Server configuration"`
}

func main() {
	// The environment variable overrides min_version in every nested TLS block.
	if err := os.Setenv("TLS_MIN_VERSION", "tls1.2"); err != nil {
		log.Fatal(err)
	}

	valid := Config{}
	if err := yaml.Load([]byte(`
client:
  tls:
    min_version:
    max_version:
server:
  tls:
    min_version:
    max_version:
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("profile loaded OK with env override applied to both TLS blocks:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: the injected env value violates the choice whitelist.
	if err := os.Setenv("TLS_MIN_VERSION", "tls1.0"); err != nil {
		log.Fatal(err)
	}

	invalid := Config{}
	err = yaml.Load([]byte(`
client:
  tls:
    min_version:
    max_version:
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("env value outside the whitelist rejected as expected:")
	fmt.Println(err)
}
