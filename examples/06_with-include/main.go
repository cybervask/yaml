// Package main demonstrates !include directive resolution during load:
// the referenced external file is inlined into the configuration tree.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Logging describes the application logging pipeline settings.
type Logging struct {
	Level  string `yaml:"level" default:"info" description:"Log level"`
	Colors bool   `yaml:"colors" description:"Colors"`
	Caller bool   `yaml:"caller" description:"Display caller information"`
	Stack  bool   `yaml:"stack" description:"Display stack information"`
}

// Server groups services exposed by the application.
type Server struct {
	Logging Logging `yaml:"logging" description:"Logging pipeline settings"`
}

// Config is the root configuration model for the application.
type Config struct {
	Server Server `yaml:"server" description:"Exposed services"`
}

func main() {
	// Valid profile: the logging subtree is loaded from the external file
	// referenced by the !include directive and merged with defaults.
	valid := Config{}
	if err := yaml.Load([]byte(`
server:
  logging: !include examples/06_with-include/logging.yaml
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: the !include directive points to a missing file.
	invalid := Config{}
	err = yaml.Load([]byte(`
server:
  logging: !include examples/06_with-include/missing.yaml
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("missing include rejected as expected:")
	fmt.Println(err)
}
