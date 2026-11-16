// Package main demonstrates source arbitration with the priority tag:
// priority:"value" promotes a non-zero field value (decoded YAML, an applied
// non-zero default tag or a pre-filled value) over a non-empty environment
// variable, while the default behavior (no priority tag or priority:"env")
// keeps the environment variable absolutely prior. A field that is still zero
// after defaults and decode — a zero default tag or an explicit zero in YAML —
// is not a value source, so the environment applies even with priority:"value".
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cybervask/yaml"
)

// Config is the root configuration model for the application.
type Config struct {
	// ValueWins has both a YAML value and an env variable; priority:"value"
	// promotes the value over the environment.
	ValueWins string `yaml:"value_wins" default:"from-default" env:"APP_VALUE_WINS" priority:"value" description:"Value wins over env with priority:\"value\""`

	// EnvWins has both sources; priority:"env" keeps the environment variable on top.
	EnvWins string `yaml:"env_wins" default:"from-default" env:"APP_ENV_WINS" priority:"env" description:"Env wins with explicit priority:\"env\""`

	// DefaultEnvWins has both sources and no priority tag; the environment
	// variable still wins (backward-compatible default behavior).
	DefaultEnvWins string `yaml:"default_env_wins" default:"from-default" env:"APP_DEFAULT_ENV_WINS" description:"Env wins without the priority tag"`

	// EnvOnly has no YAML value and no default; the single present source is
	// applied regardless of the priority tag.
	EnvOnly time.Duration `yaml:"env_only" env:"APP_ENV_ONLY" priority:"value" description:"Env applies when no value source exists"`

	// ZeroDefaultEnvWins has a zero default tag: the field is still zero after
	// defaults and decode, so with priority:"value" the environment applies as
	// the only value source.
	ZeroDefaultEnvWins int `yaml:"zero_default_env_wins" default:"0" env:"APP_ZERO_DEFAULT" priority:"value" description:"Env applies when the default tag is zero"`
}

func main() {
	// Every demonstrated field gets a non-empty environment variable.
	for _, kv := range [][2]string{
		{"APP_VALUE_WINS", "from-env"},
		{"APP_ENV_WINS", "from-env"},
		{"APP_DEFAULT_ENV_WINS", "from-env"},
		{"APP_ENV_ONLY", "5s"},
		{"APP_ZERO_DEFAULT", "9090"},
	} {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			log.Fatal(err)
		}
	}

	var cfg Config
	if err := yaml.Load([]byte(`
value_wins: from-yaml
env_wins: from-yaml
default_env_wins: from-yaml
`), &cfg); err != nil {
		log.Fatal("profile unexpectedly failed: ", err)
	}

	fmt.Println("profile loaded OK; expected winners:")
	fmt.Println(`  value_wins=from-yaml (priority:"value"), env_wins=from-env (priority:"env"),`)
	fmt.Println("  default_env_wins=from-env (no priority tag), env_only=5s (single source),")
	fmt.Println(`  zero_default_env_wins=9090 (zero default:"0" is not a value for priority:"value")`)

	out, err := yaml.Dump(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	fmt.Printf("resolved: value_wins=%s env_wins=%s default_env_wins=%s env_only=%s zero_default_env_wins=%d\n",
		cfg.ValueWins, cfg.EnvWins, cfg.DefaultEnvWins, cfg.EnvOnly, cfg.ZeroDefaultEnvWins)
}
