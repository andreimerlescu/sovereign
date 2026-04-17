package main

import (
    "bytes"
    "fmt"
    "os"
    "path/filepath"
    "text/template"
    "time"
)

// topobuilderTemplate is the text/template source for rendering a
// topobuilder-compatible YAML config file for a single sovereign planet.
//
// Each planet gets its own rendered config file. The rendered file
// is written to the planet's working directory and passed to the
// topobuilder binary via the -config flag.
//
// The template uses the same YAML structure that topobuilder expects
// as documented in the topobuilder README. Fields that sovereign does
// not need to configure are omitted or given safe defaults.
//
// Notably absent from the template:
//   Docker section -- lemmings is installed as a Go binary directly
//                     via ProvisionerLemmings, not as a Docker container
//   LoadBalancer   -- sovereign planets are not load balanced
//                     each planet runs independently
//   Wazuh          -- security monitoring is optional per deployment
//                     sovereign does not require it
const topobuilderTemplate = `
Name: {{ .Name }}
Slug: {{ .Slug }}
Domain: {{ .Domain }}
Mode: production
Email: {{ .Email }}
RollbackEnabled: false

Directories:
  Base: {{ .BaseDir }}
  Keys: {{ .KeysDir }}
  Logs: "{{ "{{" }} Directories.Base {{ "}}" }}/logs"
  Terraform: "{{ "{{" }} Directories.Base {{ "}}" }}/terraform"
  Tarballs: "{{ "{{" }} Directories.Base {{ "}}" }}/tarballs"
  Containers: "{{ "{{" }} Directories.Base {{ "}}" }}/containers"

SSH:
  Reference: {{ .SSHKeyPath }}

AWS:
  ACCESS_KEY: {{ .AWSAccessKey }}
  ACCESS_SECRET_KEY: {{ .AWSAccessSecretKey }}
  Zones:
    -
      AlwaysReplace: true
      Region: {{ .Region }}
      Zone: {{ .Zone }}
      AMI:
        ID: ami-
        OS: ubuntu/images/hvm-ssd/ubuntu-*-amd64*
        Owners: amazon
      InstanceType: {{ .InstanceType }}
      Quantity: 1
      Subnet:
        ID: {{ .SubnetID }}
        IssuePublicIPv4: true
      SecurityGroups:
        -
          GroupID: {{ .SecurityGroupID }}

EBS:
  Root:
    Size: 30
    Type: gp2
  Temp:
    Size: 10
    Type: gp2

Ports:
  Application:
    Unsecure:
      Port: 8080
    Secure:
      Port: 8443
  StatusAgent:
    Unsecure:
      Port: 18888
    Secure:
      Port: 18889

GitLab:
  GroupName: {{ .GitLabGroupName }}
  GroupID: {{ .GitLabGroupID }}
  ProjectID: {{ .GitLabProjectID }}
  RegistryPort:
    Port: 5050
  Domain: {{ .GitLabDomain }}
  DeployUsername: {{ .GitLabUsername }}
  Token: {{ .GitLabToken }}

Secrets:
  Key: {{ .SecretsKey }}
  ConvertEnvToSecret:
    - AWS_ACCESS_KEY
    - AWS_ACCESS_SECRET_KEY
`

// TopobuilderTemplateData holds the values substituted into the
// topobuilder YAML template for a single planet. All fields are
// required -- an empty field will produce an invalid topobuilder
// config and the provisioning step will fail with a clear error
// from topobuilder's own validation.
type TopobuilderTemplateData struct {
    // Planet identity
    Name   string
    Slug   string
    Domain string
    Email  string

    // Directory layout on the machine running sovereign
    BaseDir string
    KeysDir string

    // SSH access
    SSHKeyPath string

    // AWS credentials
    AWSAccessKey       string
    AWSAccessSecretKey string

    // AWS zone configuration
    Region       string
    Zone         string
    InstanceType string
    SubnetID     string
    SecurityGroupID string

    // GitLab backend for Terraform state
    GitLabGroupName string
    GitLabGroupID   int
    GitLabProjectID int
    GitLabDomain    string
    GitLabUsername  string
    GitLabToken     string

    // Secrets encryption key
    SecretsKey string
}

// renderTopobuilderYAML renders the topobuilder YAML template with
// the provided data and returns the rendered content as a string.
// Returns an error if any required field produces invalid output.
func renderTopobuilderYAML(data TopobuilderTemplateData) (string, error) {
    tmpl, err := template.New("topobuilder").Parse(topobuilderTemplate)
    if err != nil {
        return "", fmt.Errorf("parse topobuilder template: %w", err)
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", fmt.Errorf("execute topobuilder template: %w", err)
    }

    return buf.String(), nil
}

// buildTemplateData constructs a TopobuilderTemplateData for a planet
// from the sovereign config and environment. AWS credentials and GitLab
// tokens are read from environment variables following the same
// convention as topobuilder itself.
//
// Required environment variables:
//
//     AWS_ACCESS_KEY              AWS access key ID
//     AWS_ACCESS_SECRET_KEY       AWS secret access key
//     SOVEREIGN_SUBNET_ID         Subnet ID for planet instances
//     SOVEREIGN_SECURITY_GROUP    Security group ID for planet instances
//     SOVEREIGN_GITLAB_TOKEN      GitLab personal access token
//     SOVEREIGN_SECRETS_KEY       Topobuilder secrets encryption key
//     SOVEREIGN_EMAIL             Contact email for topobuilder config
//
// Optional environment variables:
//
//     SOVEREIGN_GITLAB_DOMAIN     GitLab domain (default: gitlab.com)
//     SOVEREIGN_GITLAB_USERNAME   GitLab username (default: sovereign)
//     SOVEREIGN_GITLAB_GROUP_ID   GitLab group ID (default: 1)
//     SOVEREIGN_GITLAB_PROJECT_ID GitLab project ID (default: 1)
func buildTemplateData(planet *Planet, cfg *SovereignConfig) (TopobuilderTemplateData, error) {
    // Required environment variables
    awsKey := os.Getenv("AWS_ACCESS_KEY")
    awsSecret := os.Getenv("AWS_ACCESS_SECRET_KEY")
    subnetID := os.Getenv("SOVEREIGN_SUBNET_ID")
    securityGroup := os.Getenv("SOVEREIGN_SECURITY_GROUP")
    gitlabToken := os.Getenv("SOVEREIGN_GITLAB_TOKEN")
    secretsKey := os.Getenv("SOVEREIGN_SECRETS_KEY")
    email := os.Getenv("SOVEREIGN_EMAIL")

    var missing []string
    if awsKey == "" {
        missing = append(missing, "AWS_ACCESS_KEY")
    }
    if awsSecret == "" {
        missing = append(missing, "AWS_ACCESS_SECRET_KEY")
    }
    if subnetID == "" {
        missing = append(missing, "SOVEREIGN_SUBNET_ID")
    }
    if securityGroup == "" {
        missing = append(missing, "SOVEREIGN_SECURITY_GROUP")
    }
    if gitlabToken == "" {
        missing = append(missing, "SOVEREIGN_GITLAB_TOKEN")
    }
    if secretsKey == "" {
        missing = append(missing, "SOVEREIGN_SECRETS_KEY")
    }
    if email == "" {
        missing = append(missing, "SOVEREIGN_EMAIL")
    }
    if len(missing) > 0 {
        return TopobuilderTemplateData{},
            fmt.Errorf("missing required environment variables: %v", missing)
    }

    // Optional environment variables with defaults
    gitlabDomain := os.Getenv("SOVEREIGN_GITLAB_DOMAIN")
    if gitlabDomain == "" {
        gitlabDomain = "gitlab.com"
    }
    gitlabUsername := os.Getenv("SOVEREIGN_GITLAB_USERNAME")
    if gitlabUsername == "" {
        gitlabUsername = "sovereign"
    }

    // Derive the AWS availability zone from the region
    // by appending the default zone suffix 'a'
    zone := planet.Region + "a"

    // Planet slug is used for directory and resource naming
    slug := fmt.Sprintf("sovereign-planet-%d-%d",
        planet.ID, time.Now().Unix())

    // Domain is a throwaway -- planets are not publicly accessible
    // via a domain. The domain field is required by topobuilder
    // but is not used for planet instances since there is no
    // load balancer configured.
    domain := fmt.Sprintf("planet-%d.sovereign.internal", planet.ID)

    home, err := os.UserHomeDir()
    if err != nil {
        return TopobuilderTemplateData{},
            fmt.Errorf("resolve home dir: %w", err)
    }

    baseDir := filepath.Join(home, ".sovereignd", "planets",
        fmt.Sprintf("planet-%d", planet.ID), "topobuilder")
    keysDir := filepath.Join(home, ".ssh")
    sshKeyPath := filepath.Join(keysDir, "sovereign-planet.pem")

    return TopobuilderTemplateData{
        Name:               fmt.Sprintf("Sovereign Planet %d", planet.ID),
        Slug:               slug,
        Domain:             domain,
        Email:              email,
        BaseDir:            baseDir,
        KeysDir:            keysDir,
        SSHKeyPath:         sshKeyPath,
        AWSAccessKey:       awsKey,
        AWSAccessSecretKey: awsSecret,
        Region:             planet.Region,
        Zone:               zone,
        InstanceType:       planet.InstanceType,
        SubnetID:           subnetID,
        SecurityGroupID:    securityGroup,
        GitLabGroupName:    "Sovereign",
        GitLabGroupID:      1,
        GitLabProjectID:    1,
        GitLabDomain:       gitlabDomain,
        GitLabUsername:     gitlabUsername,
        GitLabToken:        gitlabToken,
        SecretsKey:         secretsKey,
    }, nil
}

// writePlanetConfig renders the topobuilder YAML for a planet and
// writes it to the planet's working directory. Returns the path
// to the written config file.
func writePlanetConfig(planet *Planet, cfg *SovereignConfig) (string, error) {
    data, err := buildTemplateData(planet, cfg)
    if err != nil {
        return "", fmt.Errorf("planet %d: build template data: %w", planet.ID, err)
    }

    rendered, err := renderTopobuilderYAML(data)
    if err != nil {
        return "", fmt.Errorf("planet %d: render topobuilder YAML: %w", planet.ID, err)
    }

    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".sovereignd", "planets",
        fmt.Sprintf("planet-%d", planet.ID))

    if err := os.MkdirAll(dir, 0700); err != nil {
        return "", fmt.Errorf("planet %d: create config dir: %w", planet.ID, err)
    }

    configPath := filepath.Join(dir, "topobuilder.yml")
    if err := os.WriteFile(configPath, []byte(rendered), 0600); err != nil {
        return "", fmt.Errorf("planet %d: write topobuilder config: %w",
            planet.ID, err)
    }

    return configPath, nil
}​​​​​​​​​​​​​​​​
