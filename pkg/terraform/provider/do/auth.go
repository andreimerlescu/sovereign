package do

import (
	"fmt"
	"os"
)

// Credentials holds the resolved DigitalOcean authentication values.
// Populated once at construction time via Resolve().
// Never read from environment again after that point.
//
// DigitalOcean requires a single personal access token. It is passed
// to terraform subprocess calls via environment variable only. It is
// never written into HCL files.
type Credentials struct {
	Token string
}

// Resolve reads DigitalOcean credentials from environment variables
// and returns an error if any required variable is missing or empty.
//
// Required environment variables:
//
//	DIGITALOCEAN_TOKEN    DigitalOcean personal access token
func Resolve() (Credentials, error) {
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		return Credentials{},
			fmt.Errorf("do: missing required environment variable: DIGITALOCEAN_TOKEN")
	}
	return Credentials{Token: token}, nil
}
