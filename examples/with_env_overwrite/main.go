package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"Minimum TLS version"`
	MaxVersion string `yaml:"max_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MAX_VERSION" description:"Maximum TLS version"`
}

type Client struct {
	TLS TLS `yaml:"tls" description:"Client TLS configuration"`
}

type Server struct {
	TLS TLS `yaml:"tls" description:"Server TLS configuration"`
}

type Config struct {
	Client Client `yaml:"client" description:"Client configuration"`
	Server Server `yaml:"server" description:"Server configuration"`
}

func main() {
	cfg := Config{}
	data := []byte(`
client:
  tls:
    min_version:
    max_version:
server:
  tls:
    min_version:
    max_version:
`)

	// global env override client and server tls min_version both!
	os.Setenv("TLS_MIN_VERSION", "tls1.2")

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
