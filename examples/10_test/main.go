// Package main demonstrates the include path tracker combined with the
// write-back variant: include paths are discovered at runtime and the
// DumpWithInclude failure mode is illustrated.
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

// Config is the root configuration model for the application.
type Config struct {
	Server struct {
		Logging Logging `yaml:"logging"`
	} `yaml:"server" description:"Exposed services"`
}

func main() {
	// 1. Load the configuration with include path tracking.
	yaml.ResetIncludeTracker()
	includeFile := "./examples/08_write-to-includes/config.yaml"

	var cfg Config
	if err := yaml.UnmarshalFile(includeFile, &cfg); err != nil {
		log.Fatal(err)
	}

	// 2. Check the include path for a specific field.
	if p := yaml.FindIncludeFile("server.logging"); p != "" {
		fmt.Printf("OK server.logging: include path: %s\n", p)
	} else {
		fmt.Println("INFO: Logging is inline (not from !include)")
	}

	// 3. Valid dump: the struct is serialized back preserving !include structure.
	if _, err := yaml.DumpWithInclude(&cfg, yaml.WithRelativeIncludes(".")); err != nil {
		log.Fatal("valid dump unexpectedly failed: ", err)
	}

	fmt.Println("valid dump completed OK")

	// 4. Invalid dump: DumpWithInclude requires a struct value, scalars fail.
	if _, err := yaml.DumpWithInclude("not-a-struct"); err == nil {
		log.Fatal("scalar input unexpectedly passed")
	} else {
		fmt.Println("scalar input rejected as expected:")
		fmt.Println(err)
	}
}
