// Package main demonstrates atomic write-back with !include preservation:
// modified subtree values are returned to their source include files.
package main

import (
	"fmt"
	"log"
	"os"

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
	// 1. Load the configuration and track where the !include subtrees came from.
	yaml.ResetIncludeTracker()
	includeFile := "./examples/08_write-to-includes/config.yaml"

	var cfg Config
	if err := yaml.UnmarshalFile(includeFile, &cfg); err != nil {
		log.Fatal(err)
	}

	// 2. Check the tracked include path of a specific field.
	if p := yaml.FindIncludeFile("server.logging"); p != "" {
		fmt.Printf("OK server.logging: include path: %s\n", p)
	} else {
		fmt.Println("INFO: Logging is inline (not from !include)")
	}

	// 3. Modify the loaded subtree and dump the whole configuration back.
	cfg.Server.Logging.Level = "debug"

	out, err := yaml.DumpWithInclude(&cfg, yaml.WithRelativeIncludes("."))
	if err != nil {
		log.Fatal(err)
	}

	// 4. The main document keeps the !include directive; the modified data is
	// written to the source files atomically by DumpWithInclude itself.
	if err := os.WriteFile(includeFile, out, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("write-back completed, config.yaml now contains:")
	fmt.Println(string(out))
}
