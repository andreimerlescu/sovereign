package ovh

import (
	"fmt"
	"os"
)

// Credentials holds the resolved OVH and OpenStack authentication values.
// Populated once at construction time via Resolve().
// Never read from environment again after that point.
//
// OVH requires two sets of credentials:
//   - OVH API credentials for the ovh terraform provider
//   - OpenStack credentials for the openstack terraform provider
//
// Both are passed to terraform subprocess calls via environment variables.
// Neither is written into HCL files.
type Credentials struct {
	// OVH API credentials
	Endpoint          string
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string

	// OpenStack credentials
	AuthURL    string
	TenantID   string
	TenantName string
	Username   string
	Password   string
	RegionName string
}

// Resolve reads OVH and OpenStack credentials from environment variables
// and returns an error if any required variable is missing or empty.
//
// Required environment variables:
//
//	OVH_ENDPOINT              OVH API endpoint (e.g. ovh-eu, ovh-us, ovh-ca)
//	OVH_APPLICATION_KEY       OVH application key
//	OVH_APPLICATION_SECRET    OVH application secret
//	OVH_CONSUMER_KEY          OVH consumer key
//	OS_AUTH_URL               OpenStack identity endpoint
//	OS_TENANT_ID              OpenStack project ID
//	OS_TENANT_NAME            OpenStack project name
//	OS_USERNAME               OpenStack username
//	OS_PASSWORD               OpenStack password
//	OS_REGION_NAME            OpenStack region name
func Resolve() (Credentials, error) {
	creds := Credentials{
		Endpoint:          os.Getenv("OVH_ENDPOINT"),
		ApplicationKey:    os.Getenv("OVH_APPLICATION_KEY"),
		ApplicationSecret: os.Getenv("OVH_APPLICATION_SECRET"),
		ConsumerKey:       os.Getenv("OVH_CONSUMER_KEY"),
		AuthURL:           os.Getenv("OS_AUTH_URL"),
		TenantID:          os.Getenv("OS_TENANT_ID"),
		TenantName:        os.Getenv("OS_TENANT_NAME"),
		Username:          os.Getenv("OS_USERNAME"),
		Password:          os.Getenv("OS_PASSWORD"),
		RegionName:        os.Getenv("OS_REGION_NAME"),
	}

	var missing []string
	if creds.Endpoint == "" {
		missing = append(missing, "OVH_ENDPOINT")
	}
	if creds.ApplicationKey == "" {
		missing = append(missing, "OVH_APPLICATION_KEY")
	}
	if creds.ApplicationSecret == "" {
		missing = append(missing, "OVH_APPLICATION_SECRET")
	}
	if creds.ConsumerKey == "" {
		missing = append(missing, "OVH_CONSUMER_KEY")
	}
	if creds.AuthURL == "" {
		missing = append(missing, "OS_AUTH_URL")
	}
	if creds.TenantID == "" {
		missing = append(missing, "OS_TENANT_ID")
	}
	if creds.TenantName == "" {
		missing = append(missing, "OS_TENANT_NAME")
	}
	if creds.Username == "" {
		missing = append(missing, "OS_USERNAME")
	}
	if creds.Password == "" {
		missing = append(missing, "OS_PASSWORD")
	}
	if creds.RegionName == "" {
		missing = append(missing, "OS_REGION_NAME")
	}
	if len(missing) > 0 {
		return Credentials{},
			fmt.Errorf("ovh: missing required environment variables: %v", missing)
	}

	return creds, nil
}
