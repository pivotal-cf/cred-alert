package config

import (
	"cred-alert/cmdflag"
	"errors"
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
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" value-name:"PATH" required:"true"`
}

type WorkerConfig struct {
	WorkDir                     string        `yaml:"work_dir"`
	RepositoryDiscoveryInterval time.Duration `yaml:"repository_discovery_interval"`
	CredentialCounterInterval   time.Duration `yaml:"credential_counter_interval"`

	Whitelist []string `yaml:"whitelist"`

	GitHub struct {
		AccessToken    string `yaml:"access_token"`
		PrivateKeyPath string `yaml:"private_key_path"`
		PublicKeyPath  string `yaml:"public_key_path"`
	} `yaml:"github"`

	PubSub struct {
		ProjectName   string `yaml:"project_name"`
		PublicKeyPath string `yaml:"public_key_path"`
		FetchHint     struct {
			Subscription string `yaml:"subscription"`
		} `yaml:"fetch_hint"`
	} `yaml:"pubsub"`

	Metrics struct {
		SentryDSN     string `yaml:"sentry_dsn"`
		DatadogAPIKey string `yaml:"datadog_api_key"`
		Environment   string `yaml:"environment"`
	} `yaml:"metrics"`

	Slack struct {
		DefaultURL     string            `yaml:"default_webhook_url"`
		DefaultChannel string            `yaml:"default_channel"`
		TeamURLs       map[string]string `yaml:"team_webhook_urls"`
	} `yaml:"slack"`

	MySQL struct {
		Username             string `yaml:"username"`
		Password             string `yaml:"password"`
		Hostname             string `yaml:"hostname"`
		Port                 uint16 `yaml:"port"`
		DBName               string `yaml:"db_name"`
		CACertificatePath    string `yaml:"ca_certificate_path"`
		CertificatePath      string `yaml:"certificate_path"`
		PrivateKeyPath       string `yaml:"private_key_path"`
		PrivateKeyPassphrase string `yaml:"private_key_passphrase"`
	} `yaml:"mysql"`

	Identity struct {
		CACertificatePath    string `yaml:"ca_certificate_path"`
		CertificatePath      string `yaml:"certificate_path"`
		PrivateKeyPath       string `yaml:"private_key_path"`
		PrivateKeyPassphrase string `yaml:"private_key_passphrase"`
	} `yaml:"identity"`

	API struct {
		BindIP   string `yaml:"bind_ip"`
		BindPort uint16 `yaml:"bind_port"`
	} `yaml:"rpc_server"`

	Rolodex struct {
		ServerAddress string `yaml:"server_address"`
		ServerPort    uint16 `yaml:"server_port"`
	} `yaml:"rolodex"`
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
		c.Identity.CACertificatePath,
		c.Identity.CertificatePath,
		c.Identity.PrivateKeyPath,
	) {
		errs = append(errs, errors.New("all identity options required if any are set"))
	}

	if !allBlankOrAllSet(
		c.MySQL.CACertificatePath,
		c.MySQL.CertificatePath,
		c.MySQL.PrivateKeyPath,
	) {
		errs = append(errs, errors.New("all mysql tls options required if any are set"))
	}

	if !allBlankOrAllSet(
		c.PubSub.ProjectName,
		c.PubSub.FetchHint.Subscription,
		c.PubSub.PublicKeyPath,
	) {
		errs = append(errs, errors.New("all pubsub options required if any are set"))
	}

	return errs
}
