// Package main demonstrates the mincount/maxcount collection capacity
// validation rules for slices and maps.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Quantities models ticket allocation limits across collections.
type Quantities struct {
	// Tickets must contain at least 3 elements.
	Tickets []string `yaml:"tickets" validate:"mincount=3" description:"Allocated ticket identifiers"`
	// PersonalTickets must contain at most 2 entries.
	PersonalTickets map[string]string `yaml:"personal_tickets" validate:"maxcount=2" description:"Personal ticket assignments"`
}

// Config is the root configuration model for the application.
type Config struct {
	Quantities Quantities `yaml:"quantities" description:"Ticket allocation settings"`
}

func main() {
	// Valid profile: 3 tickets and 2 personal assignments satisfy the bounds.
	valid := Config{}
	if err := yaml.Load([]byte(`
quantities:
  tickets:
    - ticket1
    - ticket2
    - ticket3
  personal_tickets:
    alice: ticket1
    helen: ticket2
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: only 2 tickets (mincount=3) and 3 personal entries (maxcount=2).
	invalid := Config{}
	err = yaml.Load([]byte(`
quantities:
  tickets:
    - ticket1
    - ticket2
  personal_tickets:
    alice: ticket1
    helen: ticket2
    ivan: ticket3
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("capacity violation rejected as expected:")
	fmt.Println(err)
}
