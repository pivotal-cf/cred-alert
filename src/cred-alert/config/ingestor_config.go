package config

import (
	"cred-alert/cmdflag"
	"errors"

	yaml "gopkg.in/yaml.v2"
)

func LoadIngestorConfig(bs []byte) (*IngestorConfig, error) {
	c := &IngestorConfig{}
	err := yaml.Unmarshal(bs, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type IngestorOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" value-name:"PATH"`
}

type IngestorConfig struct {
	Port uint16 `yaml:"port"`

	GitHub struct {
		WebhookSecretTokens []string `yaml:"webhook_secret_tokens"`
	} `yaml:"github"`

	PubSub struct {
		ProjectName    string `yaml:"project_name"`
		Topic          string `yaml:"topic"`
		PrivateKeyPath string `yaml:"private_key_path"`
	} `yaml:"pubsub"`

	Metrics struct {
		SentryDSN     string `yaml:"sentry_dsn"`
		DatadogAPIKey string `yaml:"datadog_api_key"`
		Environment   string `yaml:"environment"`
	} `yaml:"metrics"`
}

func (c *IngestorConfig) Validate() []error {
	var errs []error

	if len(c.GitHub.WebhookSecretTokens) == 0 {
		errs = append(errs, errors.New("no github webhook secret tokens specified"))
	}

	if c.PubSub.ProjectName == "" {
		errs = append(errs, errors.New("no pubsub project name specified"))
	}

	if c.PubSub.Topic == "" {
		errs = append(errs, errors.New("no pubsub topic specified"))
	}

	if string(c.PubSub.PrivateKeyPath) == "" {
		errs = append(errs, errors.New("no pubsub private key specified"))
	}

	return errs
}

func (c *IngestorConfig) IsSentryConfigured() bool {
	return c.Metrics.SentryDSN != ""
}
