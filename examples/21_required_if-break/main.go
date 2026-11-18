// Package main demonstrates the break modifier of the validate tag: the
// standalone break token splits the tag into the unconditional head and the
// conditional tail, whose value rules run only when the required_if preceding
// break is met. Here the webhook endpoint url becomes mandatory — and must be
// a valid URL of a sane length — only in webhook delivery mode; pooling mode
// carries no url constraints at all.
//
// The accepted trade-off of the modifier is demonstrated explicitly: with the
// condition false the tail is skipped entirely, even for a non-empty field
// value — pooling mode accepts a garbage url silently. Configuration
// mistakes of the tail rules (minlen=abc, an empty choice list) are still
// reported regardless of the condition, so a broken declaration never hides
// behind a false condition.
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

// Webhook carries the TLS-adjacent endpoint pair. The break modifier makes
// the whole tail (url, minlen=10) conditional: the rules parsed before break
// stay unconditional, the rules after break apply only when telegram.mode is
// webhook.
type Webhook struct {
	URL string `yaml:"url" validate:"required_if=telegram.mode==webhook,break,url,minlen=10" description:"Webhook endpoint URL, mandatory in webhook mode"`
}

// Config binds the Telegram node and its webhook endpoint.
type Config struct {
	Telegram Telegram `yaml:"telegram"`
	Webhook  Webhook  `yaml:"webhook"`
}

// BrokenTailConfig carries a broken tail declaration: minlen=abc is a
// configuration mistake reported regardless of the condition — the hole
// "broken tail hidden behind a false condition" is closed.
type BrokenTailConfig struct {
	Telegram Telegram `yaml:"telegram"`
	URL      string   `yaml:"url" validate:"required_if=telegram.mode==webhook,break,minlen=abc"`
}

func main() {
	// Pooling mode with an empty url: the condition is not met, the tail
	// never joins the validation — the main break case.
	pooling := Config{}
	if err := yaml.Load([]byte(`
telegram:
  mode: pooling
`), &pooling); err != nil {
		log.Fatal("pooling mode with an empty url unexpectedly failed: ", err)
	}

	fmt.Println("pooling mode, empty url: OK (the conditional tail stays off):")
	out, err := yaml.Dump(pooling)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Webhook mode with an empty url: the condition is met, the empty field
	// reports the required error and the tail value rules (url, minlen).
	webhook := Config{}
	err = yaml.Load([]byte(`
telegram:
  mode: webhook
`), &webhook)
	if err == nil {
		log.Fatal("webhook mode with an empty url unexpectedly passed validation")
	}

	fmt.Println("webhook mode, empty url: rejected as expected:")
	fmt.Println(err)
	fmt.Println()

	// Webhook mode with a short garbage url: the condition is met and both
	// tail rules fire — the url format and the minlen limit.
	webhookShort := Config{}
	err = yaml.Load([]byte(`
telegram:
  mode: webhook
webhook:
  url: "not-a-url"
`), &webhookShort)
	if err == nil {
		log.Fatal("webhook mode with a short garbage url unexpectedly passed validation")
	}

	fmt.Println("webhook mode, short garbage url: both tail rules rejected it:")
	fmt.Println(err)
	fmt.Println()

	// Pooling mode with a garbage url: the accepted trade-off — the tail is
	// skipped entirely when the condition is false, including non-empty
	// field values. The garbage url passes silently.
	poolingGarbage := Config{}
	if err := yaml.Load([]byte(`
telegram:
  mode: pooling
webhook:
  url: "not-a-url"
`), &poolingGarbage); err != nil {
		log.Fatal("pooling mode with a garbage url unexpectedly failed: ", err)
	}

	fmt.Println("pooling mode, garbage url: OK — the accepted break trade-off (the tail is skipped even for a non-empty value):")
	out, err = yaml.Dump(poolingGarbage)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Configuration mistakes of the tail rules surface regardless of the
	// condition: minlen=abc is reported even in pooling mode, where the tail
	// value rules never run.
	brokenTail := BrokenTailConfig{}
	err = yaml.Load([]byte(`
telegram:
  mode: pooling
url: "https://hooks.example.com/notify"
`), &brokenTail)
	if err == nil {
		log.Fatal("the broken tail declaration unexpectedly passed validation")
	}

	fmt.Println("broken tail declaration (minlen=abc), pooling mode: reported regardless of the condition:")
	fmt.Println(err)
	fmt.Println()

	// Help rendering keeps the service break token undocumented: the
	// remaining segments keep their original formatting.
	fmt.Println("help output (the break token is hidden from the rule list):")
	fmt.Println(yaml.HelpStr(Config{}))
}
