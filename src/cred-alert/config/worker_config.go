package config

import (
	"cred-alert/cmdflag"
	"errors"
	"reflect"
	"time"

	yaml "gopkg.in/yaml.v2"
)

func LoadWorkerConfig(bs []byte) (*WorkerConfig, error) {
	c := &WorkerConfig{}
	err := yaml.Unmarshal(bs, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type WorkerOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" value-name:"PATH"`

	*WorkerConfig
}

type WorkerConfig struct {
	WorkDir                     string        `long:"work-dir" description:"directory to work in" value-name:"PATH" yaml:"work_dir"`
	RepositoryDiscoveryInterval time.Duration `long:"repository-discovery-interval" description:"how frequently to ask GitHub for all repos to check which ones we need to clone and dirscan" value-name:"SCAN_INTERVAL" yaml:"repository_discovery_interval"`
	CredentialCounterInterval   time.Duration `long:"credential-counter-interval" description:"how frequently to update the current count of credentials in each branch of a repository" value-name:"SCAN_INTERVAL" yaml:"credential_counter_interval"`

	Whitelist []string `short:"i" long:"ignore-pattern" description:"List of regex patterns to ignore." env:"IGNORED_PATTERNS" env-delim:"," value-name:"REGEX" yaml:"whitelist"`

	GitHub struct {
		AccessToken    string           `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN" yaml:"access_token"`
		PrivateKeyPath cmdflag.FileFlag `long:"github-private-key-path" description:"private key to use for GitHub auth" value-name:"SSH_KEY" yaml:"private_key_path"`
		PublicKeyPath  cmdflag.FileFlag `long:"github-public-key-path" description:"public key to use for GitHub auth" value-name:"SSH_KEY" yaml:"public_key_path"`
	} `group:"GitHub Options" yaml:"github"`

	PubSub struct {
		ProjectName   string           `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME" yaml:"project_name"`
		PublicKeyPath cmdflag.FileFlag `long:"pubsub-public-key" description:"path to file containing PEM-encoded, unencrypted RSA public key" yaml:"public_key_path"`
		FetchHint     struct {
			Subscription string `long:"fetch-hint-pubsub-subscription" description:"PubSub Topic receive messages from" value-name:"NAME" yaml:"subscription"`
		} `group:"PubSub Fetch Hint Options" yaml:"fetch_hint"`
	} `group:"PubSub Options" yaml:"pubsub"`

	Metrics struct {
		SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN" yaml:"sentry_dsn"`
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY" yaml:"datadog_api_key"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" yaml:"environment"`
	} `group:"Metrics Options" yaml:"metrics"`

	Slack struct {
		WebhookURL string `long:"slack-webhook-url" description:"Slack webhook URL" env:"SLACK_WEBHOOK_URL" value-name:"WEBHOOK" yaml:"webhook_url"`
	} `group:"Slack Options" yaml:"slack"`

	MySQL struct {
		Username string `long:"mysql-username" description:"MySQL username" value-name:"USERNAME" yaml:"username"`
		Password string `long:"mysql-password" description:"MySQL password" value-name:"PASSWORD" yaml:"password"`
		Hostname string `long:"mysql-hostname" description:"MySQL hostname" value-name:"HOSTNAME" yaml:"hostname"`
		Port     uint16 `long:"mysql-port" description:"MySQL port" value-name:"PORT" yaml:"port"`
		DBName   string `long:"mysql-dbname" description:"MySQL database name" value-name:"DBNAME" yaml:"db_name"`
	} `group:"MySQL Options" yaml:"mysql"`

	RPC struct {
		ClientCACertificatePath cmdflag.FileFlag `long:"rpc-server-client-ca" description:"Path to client CA certificate" yaml:"client_ca_certificate_path"`
		CertificatePath         cmdflag.FileFlag `long:"rpc-server-cert" description:"Path to RPC server certificate" yaml:"certificate_path"`
		PrivateKeyPath          cmdflag.FileFlag `long:"rpc-server-private-key" description:"Path to RPC server private key" yaml:"private_key_path"`
		PrivateKeyPassphrase    string           `long:"rpc-server-private-key-passphrase" description:"Passphrase for the RPC server private key, if encrypted" yaml:"private_key_passphrase"`
		BindIP                  string           `long:"rpc-server-bind-ip" description:"IP address on which to listen for RPC traffic." yaml:"bind_ip"`
		BindPort                uint16           `long:"rpc-server-bind-port" description:"Port on which to listen for RPC traffic." yaml:"bind_port"`
	} `group:"RPC Options" yaml:"rpc_server"`
}

func (c *WorkerConfig) Validate() []error {
	var errs []error

	if c.WorkDir == "" {
		errs = append(errs, errors.New("no workdir specified"))
	}

	if c.MySQL.Username == "" {
		errs = append(errs, errors.New("no mysql username specified"))
	}

	if c.MySQL.Hostname == "" {
		errs = append(errs, errors.New("no mysql hostname specified"))
	}

	if c.MySQL.DBName == "" {
		errs = append(errs, errors.New("no mysql db name specified"))
	}

	if !allBlankOrAllSet(
		string(c.RPC.ClientCACertificatePath),
		string(c.RPC.CertificatePath),
		string(c.RPC.PrivateKeyPath),
	) {
		errs = append(errs, errors.New("all rpc options required if any are set"))
	}

	if !allBlankOrAllSet(
		string(c.PubSub.ProjectName),
		string(c.PubSub.FetchHint.Subscription),
		string(c.PubSub.PublicKeyPath),
	) {
		errs = append(errs, errors.New("all pubsub options required if any are set"))
	}

	return errs
}

func (c *WorkerConfig) IsRPCConfigured() bool {
	return allSet(
		string(c.RPC.ClientCACertificatePath),
		string(c.RPC.CertificatePath),
		string(c.RPC.PrivateKeyPath),
	)
}

func (c *WorkerConfig) IsPubSubConfigured() bool {
	return allSet(
		c.PubSub.ProjectName,
		c.PubSub.FetchHint.Subscription,
		string(c.PubSub.PublicKeyPath),
	)
}

func (c *WorkerConfig) Merge(other *WorkerConfig) error {
	src := reflect.ValueOf(other).Elem()
	dst := reflect.ValueOf(c).Elem()

	return merge(dst, src)
}
