package aws

import (
	"fmt"
	"os"
)

// Credentials holds the resolved AWS authentication values.
// Populated once at construction time via Resolve().
// Never read from environment again after that point.
type Credentials struct {
	AccessKey       string
	AccessSecretKey string
}

// Resolve reads AWS credentials from environment variables and
// returns an error if any required variable is missing or empty.
//
// Required environment variables:
//
//	AWS_ACCESS_KEY
//	AWS_ACCESS_SECRET_KEY
func Resolve() (Credentials, error) {
	key := os.Getenv("AWS_ACCESS_KEY")
	secret := os.Getenv("AWS_ACCESS_SECRET_KEY")

	var missing []string
	if key == "" {
		missing = append(missing, "AWS_ACCESS_KEY")
	}
	if secret == "" {
		missing = append(missing, "AWS_ACCESS_SECRET_KEY")
	}
	if len(missing) > 0 {
		return Credentials{}, fmt.Errorf("aws: missing required environment variables: %v", missing)
	}

	return Credentials{
		AccessKey:       key,
		AccessSecretKey: secret,
	}, nil
}
