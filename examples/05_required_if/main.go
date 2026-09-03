// Package main demonstrates cross-field required_if validation combined with
// the endpoint and url rules.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Client holds the parameters of the outgoing connection endpoint.
type Client struct {
	// Addr must be a valid URL and becomes mandatory when Role equals "client".
	Addr string `yaml:"addr" validate:"url,required_if=Role:client" description:"Address and port to connect to. Dual-stack supported."`
}

// Server holds the parameters of the incoming listener endpoint.
type Server struct {
	// Addr must be a host:port endpoint; the default only applies when the
	// field is left empty in YAML.
	Addr string `yaml:"addr" default:"127.0.0.1:8088" validate:"endpoint,required_if=Role:server" description:"Address and port to connect to. Dual-stack supported."`
}

// Config is the root configuration model binding the role and both endpoints.
type Config struct {
	Client    Client `yaml:"client" description:"Outgoing client settings"`
	Server    Server `yaml:"server" description:"Incoming server settings"`
	Role      string `yaml:"role" description:"Active application role"`
	ActorRole string `yaml:"actor_role" validate:"required_if=role:!server" description:"Actor role, mandatory unless role is server"`
}

func main() {
	// Valid profile: role=server makes the server endpoint required (satisfied
	// by the tag default), and the negated condition passes with a filled actor_role.
	valid := Config{}
	if err := yaml.Load([]byte(`
server:
  addr: "[::]:8080"
client:
  addr:
role: "server"
actor_role: "admin"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: role=client makes the client endpoint mandatory, but it
	// is left empty (no default is declared for it), and the empty actor_role
	// violates required_if=role:!server because role differs from "server".
	invalid := Config{}
	err = yaml.Load([]byte(`
server:
  addr: "[::]:8080"
client:
  addr:
role: "client"
actor_role:
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("conditional requirement violation rejected as expected:")
	fmt.Println(err)
}
