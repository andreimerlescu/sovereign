package aws

import (
	"fmt"
	"os"
)

// Credentials holds the resolved AWS authentication values.
// Populated once at construction time via Resolve().
// Never read from environment again after that point.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// Resolve reads AWS credentials from the standard AWS environment
// variables and returns an error if any required variable is missing
// or empty.
//
// Required environment variables:
//
//	AWS_ACCESS_KEY_ID        standard AWS access key identifier
//	AWS_SECRET_ACCESS_KEY    standard AWS secret access key
//
// These names match what the AWS CLI, SDK, and Terraform AWS provider
// all read natively, so operators who already have AWS tooling
// configured will not need to set any additional variables.
func Resolve() (Credentials, error) {
	keyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_SECRET_ACCESS_KEY")

	var missing []string
	if keyID == "" {
		missing = append(missing, "AWS_ACCESS_KEY_ID")
	}
	if secret == "" {
		missing = append(missing, "AWS_SECRET_ACCESS_KEY")
	}
	if len(missing) > 0 {
		return Credentials{}, fmt.Errorf("aws: missing required environment variables: %v", missing)
	}

	return Credentials{
		AccessKeyID:     keyID,
		SecretAccessKey: secret,
	}, nil
}
