// Package main provides example
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type Client struct {
	Addr string `yaml:"addr" validate:"url,required_if=Role:client"  description:"Address and port to connect to. Dual-stack supported."`
}

type Server struct {
	Addr string `yaml:"addr" default:"127.0.0.1:8088" validate:"endpoint,require-if=Role:server"  description:"Address and port to connect to. Dual-stack supported."`
}

type Config struct {
	Client    Client `yaml:"client"`
	Server    Server `yaml:"server"`
	Role      string `yaml:"role"`
	ActorRole string `yaml:"actor_role" validate:"required_if=role:!server"`
}

func main() {
	cfg := Config{}
	data := []byte(`
server:
  addr: "[::]:8080"
client:
  addr:
role: "server"
actor_role:
`)

	err := yaml.Load(data, &cfg)
	if err != nil {
		log.Fatal("parse config:", err.Error())
	}

	out, err := yaml.Dump(cfg)
	if err != nil {
		log.Fatal("dump config:", err.Error())
	}

	fmt.Println(string(out))
}
