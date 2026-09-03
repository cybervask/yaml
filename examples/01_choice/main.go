// Package main demonstrates the choice validation rule in both whitelist and
// blacklist modes, including quoted tokens that embed literal commas.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// ACL models an access control policy with restricted order values.
type ACL struct {
	// Order must match one of the two quoted whitelist tokens; the quotes keep
	// the embedded commas from being treated as separators.
	Order string `yaml:"order" validate:"choice='allow,deny','deny,allow'" description:"Permitted execution order"`
	// Order1 must be exactly "allow" or "deny" (plain whitelist tokens).
	Order1 string `yaml:"order1" validate:"choice=allow,deny" description:"Single action to execute"`
}

func main() {
	// Valid profile: both fields match their whitelists.
	valid := ACL{}
	if err := yaml.Load([]byte(`
order: allow,deny
order1: allow
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: order1 is outside its whitelist.
	invalid := ACL{}
	err = yaml.Load([]byte(`
order: allow,deny
order1: revoke
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("invalid profile rejected as expected:")
	fmt.Println(err)
}
