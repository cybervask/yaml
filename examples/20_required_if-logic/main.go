// Package main demonstrates boolean expressions inside required_if: several
// condition atoms joined with "&&" (AND) and "||" (OR), grouped with
// parentheses. Standard C/Go precedence applies — parentheses bind tighter
// than "&&", which binds tighter than "||". An operand carrying "&&" or
// "||" must be quoted; parentheses inside an operand (after the comparison
// operator) need no quotes. The deprecated colon atoms ("key:value") may be
// mixed with the canonical ones.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Telegram switches the notification delivery mode.
type Telegram struct {
	Mode string `yaml:"mode" validate:"choice=webhook,pooling" description:"Delivery mode: webhook or pooling"`
}

// Webhook carries the TLS material. key_file and cert_file become mandatory
// through required_if boolean expressions: the key is required once the
// certificate is provided, the certificate is required for HTTPS URLs or
// once the key is provided.
type Webhook struct {
	URL      string `yaml:"url" description:"Webhook endpoint URL"`
	KeyFile  string `yaml:"key_file" validate:"required_if=telegram.mode==webhook&&webhook.url~=^https:&&webhook.cert_file^=^$" description:"TLS key, mandatory once the certificate is provided over HTTPS"`
	CertFile string `yaml:"cert_file" validate:"required_if=telegram.mode==webhook&&(webhook.url~=^https:||webhook.key_file^=^$)" description:"TLS certificate, mandatory for HTTPS webhooks or once the key is provided"`
}

// Config binds the Telegram node and its webhook TLS pair.
type Config struct {
	Telegram Telegram `yaml:"telegram"`
	Webhook  Webhook  `yaml:"webhook"`
}

func main() {
	// Valid profile: webhook mode over HTTPS with the full TLS pair — both
	// expressions are met and both fields are filled.
	valid := Config{}
	if err := yaml.Load([]byte(`
telegram:
  mode: webhook
webhook:
  url: "https://hooks.example.com/notify"
  key_file: "/etc/tls/key.pem"
  cert_file: "/etc/tls/cert.pem"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: the certificate is provided over HTTPS but the key
	// is missing — key_file's three-atom conjunction fires.
	invalid := Config{}
	err = yaml.Load([]byte(`
telegram:
  mode: webhook
webhook:
  url: "https://hooks.example.com/notify"
  cert_file: "/etc/tls/cert.pem"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("missing TLS key rejected as expected:")
	fmt.Println(err)

	// Invalid profile: HTTPS webhook without any TLS material — cert_file's
	// parenthesized OR fires.
	invalid2 := Config{}
	err = yaml.Load([]byte(`
telegram:
  mode: webhook
webhook:
  url: "https://hooks.example.com/notify"
`), &invalid2)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("missing TLS certificate rejected as expected:")
	fmt.Println(err)

	// Pooling mode disables both requirements entirely.
	pooling := Config{}
	if err := yaml.Load([]byte(`
telegram:
  mode: pooling
`), &pooling); err != nil {
		log.Fatal("pooling profile unexpectedly failed: ", err)
	}

	fmt.Println("pooling profile loaded OK:")
	out, err = yaml.Dump(pooling)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))
}
