package config

import (
	"errors"
	"time"
)

type WorkerOpts struct {
	*WorkerConfig
}

type WorkerConfig struct {
	LogLevel                    string        `long:"log-level" description:"log level to use"`
	WorkDir                     string        `long:"work-dir" description:"directory to work in" value-name:"PATH"`
	RepositoryDiscoveryInterval time.Duration `long:"repository-discovery-interval" description:"how frequently to ask GitHub for all repos to check which ones we need to clone and dirscan" value-name:"SCAN_INTERVAL" default:"1h"`
	ChangeDiscoveryInterval     time.Duration `long:"change-discovery-interval" description:"how frequently to fetch changes for repositories on disk and scan the changes" value-name:"SCAN_INTERVAL" default:"1h"`
	MinFetchInterval            time.Duration `long:"min-fetch-interval" description:"the minimum frequency to fetch changes for repositories on disk and scan the changes" value-name:"MIN_FETCH_INTERVAL" default:"6h"`
	MaxFetchInterval            time.Duration `long:"max-fetch-interval" description:"the maximum frequency to fetch changes for repositories on disk and scan the changes" value-name:"MAX_FETCH_INTERVAL" default:"168h"`
	CredentialCounterInterval   time.Duration `long:"credential-counter-interval" description:"how frequently to update the current count of credentials in each branch of a repository" value-name:"SCAN_INTERVAL" default:"24h"`

	Whitelist []string `short:"i" long:"ignore-pattern" description:"List of regex patterns to ignore." env:"IGNORED_PATTERNS" env-delim:"," value-name:"REGEX"`

	RPCBindIP   string `long:"rpc-server-bind-ip" default:"0.0.0.0" description:"IP address on which to listen for RPC traffic."`
	RPCBindPort uint16 `long:"rpc-server-bind-port" default:"50051" description:"Port on which to listen for RPC traffic."`

	GitHub struct {
		AccessToken    string `short:"a" long:"access-token" description:"github api access token" env:"GITHUB_ACCESS_TOKEN" value-name:"TOKEN"`
		PrivateKeyPath string `long:"github-private-key-path" description:"private key to use for GitHub auth" value-name:"SSH_KEY"`
		PublicKeyPath  string `long:"github-public-key-path" description:"public key to use for GitHub auth" value-name:"SSH_KEY"`
	} `group:"GitHub Options"`

	PubSub struct {
		ProjectName string `long:"pubsub-project-name" description:"GCP Project Name" value-name:"NAME"`
		PublicKey   string `long:"pubsub-public-key" description:"path to file containing PEM-encoded, unencrypted RSA public key"`
		FetchHint   struct {
			Subscription string `long:"fetch-hint-pubsub-subscription" description:"PubSub Topic receive messages from" value-name:"NAME"`
		} `group:"PubSub Fetch Hint Options"`
	} `group:"PubSub Options"`

	Metrics struct {
		SentryDSN     string `long:"sentry-dsn" description:"DSN to emit to Sentry with" env:"SENTRY_DSN" value-name:"DSN"`
		DatadogAPIKey string `long:"datadog-api-key" description:"key to emit to datadog" env:"DATADOG_API_KEY" value-name:"KEY"`
		Environment   string `long:"environment" description:"environment tag for metrics" env:"ENVIRONMENT" value-name:"NAME" default:"development"`
	} `group:"Metrics Options"`

	Slack struct {
		WebhookURL string `long:"slack-webhook-url" description:"Slack webhook URL" env:"SLACK_WEBHOOK_URL" value-name:"WEBHOOK"`
	} `group:"Slack Options"`

	MySQL struct {
		Username string `long:"mysql-username" description:"MySQL username" value-name:"USERNAME"`
		Password string `long:"mysql-password" description:"MySQL password" value-name:"PASSWORD"`
		Hostname string `long:"mysql-hostname" description:"MySQL hostname" value-name:"HOSTNAME"`
		Port     uint16 `long:"mysql-port" description:"MySQL port" value-name:"PORT" default:"3306"`
		DBName   string `long:"mysql-dbname" description:"MySQL database name" value-name:"DBNAME"`
	} `group:"MySQL Options"`

	RPC struct {
		ClientCACertificate string `long:"rpc-server-client-ca" description:"Path to client CA certificate"`
		Certificate         string `long:"rpc-server-cert" description:"Path to RPC server certificate"`
		PrivateKey          string `long:"rpc-server-private-key" description:"Path to RPC server private key"`
	} `group:"RPC Options"`
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
		c.RPC.ClientCACertificate,
		c.RPC.Certificate,
		c.RPC.PrivateKey,
	) {
		errs = append(errs, errors.New("all rpc options required if any are set"))
	}

	if !allBlankOrAllSet(
		c.PubSub.ProjectName,
		c.PubSub.FetchHint.Subscription,
		c.PubSub.PublicKey,
	) {
		errs = append(errs, errors.New("all pubsub options required if any are set"))
	}

	return errs
}

func (c *WorkerConfig) IsRPCConfigured() bool {
	return allSet(
		c.RPC.ClientCACertificate,
		c.RPC.Certificate,
		c.RPC.PrivateKey,
	)
}

func (c *WorkerConfig) IsPubSubConfigured() bool {
	return allSet(
		c.PubSub.ProjectName,
		c.PubSub.FetchHint.Subscription,
		c.PubSub.PublicKey,
	)
}

func allSet(xs ...string) bool {
	for i := range xs {
		if xs[i] == "" {
			return false
		}
	}

	return true
}

func allBlankOrAllSet(xs ...string) bool {
	var blanks int
	for i := range xs {
		if xs[i] == "" {
			blanks++
		}
	}

	return blanks == len(xs) || blanks == 0
}
