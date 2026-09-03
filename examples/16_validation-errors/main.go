// Package main demonstrates aggregated validation: multiple rules of different
// kinds are checked in one pass and all violations are reported together.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Logging struct {
	Level string `yaml:"level" validate:"choice=debug,info,warn" description:"Log level"`
}

type Server struct {
	BindEndpoint string `yaml:"bind_endpoint" validate:"endpoint" description:"host:port bind address"`
	Workers      int    `yaml:"workers" validate:"min=1,max=64" description:"Worker pool size"`
}

type Config struct {
	Env        string   `yaml:"env" validate:"choice=dev,stage,prod" description:"Deployment environment"`
	Logging    Logging  `yaml:"logging"`
	Server     Server   `yaml:"server"`
	AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"Whitelisted network remotes"`
	AdminEmail string   `yaml:"admin_email" validate:"regexp=^[^@]+@[^@]+$" description:"Administrator contact"`
}

func main() {
	// Valid profile: every field satisfies its constraints.
	valid := Config{}
	if err := yaml.Load([]byte(`
env: "prod"
logging:
  level: "info"
server:
  bind_endpoint: "127.0.0.1:443"
  workers: 8
allowed_ips:
  - "192.168.1.1"
admin_email: "admin@cybervask.net"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Invalid profile: six violations of six different rule kinds at once.
	// The engine collects them all instead of stopping at the first failure.
	invalid := Config{}
	err = yaml.Load([]byte(`
env: "testing"            # choice: outside the whitelist
logging:
  level: "verbose"        # choice: outside the nested whitelist
server:
  bind_endpoint: "google.com" # endpoint: missing host:port
  workers: 100            # max: above the inclusive upper bound
allowed_ips: []           # mincount: empty collection
admin_email: "not-an-email" # regexp: pattern mismatch
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("invalid profile rejected with all violations collected in one report:")
	fmt.Println(err)
}
