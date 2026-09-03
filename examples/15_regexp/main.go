// Package main demonstrates the regexp validation rule, including patterns
// containing commas (kept intact because they are not followed by a known rule).
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type DNS struct {
	// Zone must match a lowercase two to four letter pattern.
	Zone string `yaml:"zone" validate:"regexp=^[a-z]{2,4}$" description:"DNS zone label"`
	// List must match a comma-separated pair pattern; the embedded comma is
	// kept as part of the pattern because no known rule follows it.
	List string `yaml:"list" validate:"regexp=^[a-z]+,[a-z]+$" description:"Comma-separated labels"`
	// Servers applies the pattern to every element of the slice recursively.
	Servers []string `yaml:"servers" validate:"regexp=^db-[0-9]+$" description:"Server name pattern"`
}

type Config struct {
	DNS DNS `yaml:"dns"`
}

func main() {
	// Valid profile: zone, quoted comma pattern and all slice elements match.
	valid := Config{}
	if err := yaml.Load([]byte(`
dns:
  zone: "dev"
  list: "primary,secondary"
  servers:
    - "db-01"
    - "db-02"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Invalid profile: uppercase zone, comma pattern mismatch and a bad server name.
	invalid := Config{}
	err = yaml.Load([]byte(`
dns:
  zone: "DEV"
  list: "primary"
  servers:
    - "db-01"
    - "cache-02"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("invalid profile rejected as expected:")
	fmt.Println(err)
}
