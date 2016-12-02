package config

import (
	"cred-alert/cmdflag"
	"errors"
	"reflect"

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

type IngestorGitHub struct {
	WebhookSecretTokens []string `short:"w" long:"github-webhook-secret-token" description:"github webhook secret token" env:"GITHUB_WEBHOOK_SECRET_TOKENS" env-delim:"," value-name:"TOKENS" yaml:"webhook_secret_tokens"`
}

type IngestorPubSub struct {
	ProjectName    string           `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME" yaml:"project_name"`
	Topic          string           `long:"pubsub-topic" description:"PubSub Topic to send message to" value-name:"NAME" yaml:"topic"`
	PrivateKeyPath cmdflag.FileFlag `long:"pubsub-private-key" description:"path to file containing PEM-encoded, unencrypted RSA private key" yaml:"private_key_path"`
}

type IngestorMetrics struct {
	SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN" yaml:"sentry_dsn"`
	DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY" yaml:"datadog_api_key"`
	Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development" yaml:"environment"`
}

type IngestorOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" value-name:"PATH"`

	*IngestorConfig
}

type IngestorConfig struct {
	Port uint16 `short:"p" long:"port" description:"the port to listen on" default:"8080" env:"PORT" value-name:"PORT" yaml:"port"`

	GitHub  IngestorGitHub  `group:"GitHub Options" yaml:"github"`
	PubSub  IngestorPubSub  `group:"PubSub Options" yaml:"pubsub"`
	Metrics IngestorMetrics `group:"Metrics Options" yaml:"metrics"`
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

func (c *IngestorConfig) Merge(other *IngestorConfig) error {
	src := reflect.ValueOf(other).Elem()
	dst := reflect.ValueOf(c).Elem()

	return merge(dst, src)
}
