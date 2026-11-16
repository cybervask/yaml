// Package main demonstrates the canonical required_if condition operators:
// "==" and "!=" compare values case-insensitively, "~=" matches a regexp
// pattern and "^=" anti-matches one. The deprecated colon form
// ("required_if=role:prod" / "required_if=role:!prod") still works with the
// same semantics plus the empty/not_empty macros; prefer the canonical
// operators in new configurations.
package main

import (
	"fmt"
	"log"

	"github.com/cybervask/yaml"
)

// Config binds every canonical required_if operator to its own guard field.
type Config struct {
	Role string `yaml:"role" description:"Active application role"`

	// APIToken is mandatory when role equals "prod" (case-insensitive).
	APIToken string `yaml:"api_token" validate:"required_if=role==prod" description:"API token, mandatory in production"`
	// DebugFlag is mandatory when role differs from "prod".
	DebugFlag string `yaml:"debug_flag" validate:"required_if=role!=prod" description:"Debug flag, mandatory outside production"`
	// RegionLock is mandatory when role matches ^(prod|stage)$.
	RegionLock string `yaml:"region_lock" validate:"required_if=role~='^(prod|stage)$'" description:"Region lock, mandatory for prod/stage roles"`
	// BackupPlan is mandatory when role does NOT match ^prod.
	BackupPlan string `yaml:"backup_plan" validate:"required_if=role^=^prod" description:"Backup plan, mandatory unless the role matches ^prod"`
}

func main() {
	// Valid profile: role=prod satisfies every operator consistently —
	// api_token (==), region_lock (~=) are filled; debug_flag (!=) and
	// backup_plan (^=) conditions are not met at all.
	valid := Config{}
	if err := yaml.Load([]byte(`
role: "prod"
api_token: "tok-123"
region_lock: "eu-central-1"
`), &valid); err != nil {
		log.Fatal("valid profile unexpectedly failed: ", err)
	}

	fmt.Println("valid profile loaded OK:")
	out, err := yaml.Dump(valid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(out))

	// Invalid profile: role=dev makes debug_flag (!=) and backup_plan (^=)
	// mandatory, and both are left empty.
	invalid := Config{}
	err = yaml.Load([]byte(`
role: "dev"
`), &invalid)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("!= and ^= violations rejected as expected:")
	fmt.Println(err)

	// Invalid profile: role=stage matches ^(prod|stage)$, so region_lock
	// (~=) becomes mandatory and is left empty.
	invalid2 := Config{}
	err = yaml.Load([]byte(`
role: "stage"
`), &invalid2)
	if err == nil {
		log.Fatal("invalid profile unexpectedly passed validation")
	}

	fmt.Println("~= violation rejected as expected:")
	fmt.Println(err)
}
