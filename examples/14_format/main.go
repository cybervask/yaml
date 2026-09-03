// Package main demonstrates the format validation rule family:
// ip, ipv4, ipv6, uuid and hex.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Network struct {
	// BindIP accepts both IPv4 and IPv6 addresses.
	BindIP string `yaml:"bind_ip" validate:"format=ip" description:"Bind address (IPv4 or IPv6)"`
	// Broadcast must be a legacy IPv4 address.
	Broadcast string `yaml:"broadcast" validate:"format=ipv4" description:"IPv4 broadcast address"`
	// Multicast must be a modern IPv6 address.
	Multicast string `yaml:"multicast" validate:"format=ipv6" description:"IPv6 multicast address"`
	// NodeID must be a RFC-formatted UUID.
	NodeID string `yaml:"node_id" validate:"format=uuid" description:"Node unique identity"`
	// SessionKey must be a hex string with an even number of digits.
	SessionKey string `yaml:"session_key" validate:"format=hex" description:"Session key in hexadecimal"`
}

type Config struct {
	Network Network `yaml:"network" description:"Network settings"`
}

func main() {
	// Valid profile: every field satisfies its format rule.
	valid := Config{}
	if err := yaml.Load([]byte(`
network:
  bind_ip: "192.168.1.1"
  broadcast: "10.255.255.255"
  multicast: "ff02::1"
  node_id: "550e8400-e29b-41d4-a716-446655440000"
  session_key: "deadBEEF"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Invalid profile: ipv4 address in the ipv6 field, ipv6 address in the ipv4
	// field, hostname instead of an ip, a malformed UUID, and hex values with
	// an odd digit count and a non-hex character.
	invalid := Config{}
	err = yaml.Load([]byte(`
network:
  bind_ip: "localhost"
  broadcast: "ff02::1"
  multicast: "192.168.1.1"
  node_id: "not-a-uuid"
  session_key: "abc"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("invalid profile rejected as expected:")
	fmt.Println(err)
}
