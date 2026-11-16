// Package main demonstrates cross-field required_if validation with the
// canonical operators (== equality, != inequality, ~= regexp match and
// ^= regexp anti-match) combined with the endpoint and url rules.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Client holds the parameters of the outgoing connection endpoint.
type Client struct {
	// Addr must be a valid URL and becomes mandatory when Role equals "client".
	Addr string `yaml:"addr" validate:"url,required_if=Role==client" description:"Address and port to connect to. Dual-stack supported."`
}

// Server holds the parameters of the incoming listener endpoint.
type Server struct {
	// Addr must be a host:port endpoint; the default only applies when the
	// field is left empty in YAML.
	Addr string `yaml:"addr" default:"127.0.0.1:8088" validate:"endpoint,required_if=Role==server" description:"Address and port to connect to. Dual-stack supported."`
}

// Config is the root configuration model binding the role and both endpoints.
type Config struct {
	Client    Client `yaml:"client" description:"Outgoing client settings"`
	Server    Server `yaml:"server" description:"Incoming server settings"`
	Role      string `yaml:"role" description:"Active application role"`
	ActorRole string `yaml:"actor_role" validate:"required_if=role!=server" description:"Actor role, mandatory unless role is server"`

	// ReleaseTicket is mandatory when the role matches ^(prod|stage)$.
	ReleaseTicket string `yaml:"release_ticket" validate:"required_if=role~='^(prod|stage)$'" description:"Release ticket, mandatory for prod/stage roles"`
	// FeatureToggle is mandatory when the role does NOT match ^prod.
	FeatureToggle string `yaml:"feature_toggle" validate:"required_if=role^=^prod" description:"Feature toggle, mandatory unless the role matches ^prod"`
}

func main() {
	// Valid profile: role=server makes the server endpoint required (satisfied
	// by the tag default), the negated condition passes with a filled actor_role,
	// and since "server" does not match ^prod, the ^= anti-match makes
	// feature_toggle mandatory (filled). "server" does not match ^(prod|stage)$
	// either, so release_ticket is not required at all.
	valid := Config{}
	if err := yaml.Load([]byte(`
server:
  addr: "[::]:8080"
client:
  addr:
role: "server"
actor_role: "admin"
feature_toggle: "flag-x"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Regexp-suppressed profile: role=production matches ^prod, so the ^= anti-
	// match no longer makes feature_toggle required (left empty), and
	// "production" does not match ^(prod|stage)$ either, so release_ticket is
	// not required. Only actor_role stays mandatory (role != server).
	suppressed := Config{}
	if err := yaml.Load([]byte(`
server:
  addr: "[::]:8080"
role: "production"
actor_role: "admin"
`), &suppressed); err != nil {
		log.Fatal("regexp-suppressed profile unexpectedly failed: ", err)
	}

	fmt.Println("regexp match/anti-match suppressed requirements loaded OK (no dump)")

	// Invalid profile: role=stage matches ^(prod|stage)$, so the ~= matcher
	// makes release_ticket mandatory while it is left empty.
	invalid := Config{}
	err = yaml.Load([]byte(`
server:
  addr: "[::]:8080"
client:
  addr:
role: "stage"
actor_role: "admin"
feature_toggle: "flag-x"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("~= regexp requirement violation rejected as expected:")
	fmt.Println(err)
}
