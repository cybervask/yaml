// Package main demonstrates the minlen/maxlen string length validation rules.
// Length is counted in Unicode runes, not bytes.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

type User struct {
	// Login must be 3..20 runes long.
	Login string `yaml:"login" validate:"minlen=3,maxlen=20" description:"User login name"`
	// Nickname may contain non-ASCII characters: rune counting keeps CJK names valid.
	Nickname string `yaml:"nickname" validate:"minlen=2,maxlen=12" description:"Display nickname (rune-aware)"`
}

type Config struct {
	User User `yaml:"user"`
}

func main() {
	// Valid profile: latin login and a CJK nickname (6 runes, 18 bytes) both pass.
	valid := Config{}
	if err := yaml.Load([]byte(`
user:
  login: "cybervask"
  nickname: "赛博面具"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}
	fmt.Println("valid profile loaded OK (CJK nickname counts as 4 runes, not 12 bytes):")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Invalid profile: login "go" is 2 runes < minlen 3, nickname is 13 runes > maxlen 12.
	invalid := Config{}
	err = yaml.Load([]byte(`
user:
  login: "go"
  nickname: "very-long-nickname"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}
	fmt.Println("invalid profile rejected as expected:")
	fmt.Println(err)
}
