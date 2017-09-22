package rolodex

import (
	"errors"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

func LoadConfig(configPath string) (*Config, error) {
	bs, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	c := &Config{}
	err = yaml.Unmarshal(bs, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type Config struct {
	RepositoryPath string `yaml:"repository_path"`
	RepositoryURL  string `yaml:"repository_url"`

	GitPath string `yaml:"git_path"`

	GitHub struct {
		PrivateKeyPath string `yaml:"private_key_path"`
		PublicKeyPath  string `yaml:"public_key_path"`
	} `yaml:"github"`

	Metrics struct {
		SentryDSN         string `yaml:"sentry_dsn"`
		DatadogAPIKey     string `yaml:"datadog_api_key"`
		Environment       string `yaml:"environment"`
		HoneycombWriteKey string `yaml:"honeycomb_write_key"`
	} `yaml:"metrics"`

	RPC struct {
		BindIP   string `yaml:"bind_ip"`
		BindPort uint16 `yaml:"bind_port"`

		CACertificatePath    string `yaml:"ca_certificate_path"`
		CertificatePath      string `yaml:"certificate_path"`
		PrivateKeyPath       string `yaml:"private_key_path"`
		PrivateKeyPassphrase string `yaml:"private_key_passphrase"`
	} `yaml:"rpc_server"`
}

func (c *Config) Validate() []error {
	var errs []error

	if c.RepositoryPath == "" {
		errs = append(errs, errors.New("no repository path specified"))
	}

	if c.RepositoryURL == "" {
		errs = append(errs, errors.New("no repository URL specified"))
	}

	if c.GitHub.PrivateKeyPath == "" {
		errs = append(errs, errors.New("no GitHub private key specified"))
	}

	if c.GitHub.PublicKeyPath == "" {
		errs = append(errs, errors.New("no GitHub public key specified"))
	}

	if c.RPC.BindIP == "" {
		errs = append(errs, errors.New("no bind IP specified"))
	}

	if c.RPC.BindPort == 0 {
		errs = append(errs, errors.New("no bind port specified"))
	}

	if c.RPC.CACertificatePath == "" {
		errs = append(errs, errors.New("no CA certificate path specified"))
	}

	if c.RPC.CertificatePath == "" {
		errs = append(errs, errors.New("no certificate specified"))
	}

	if c.RPC.PrivateKeyPath == "" {
		errs = append(errs, errors.New("no private key path specified"))
	}

	if c.Metrics.SentryDSN == "" {
		errs = append(errs, errors.New("no sentry dsn specified"))
	}

	if c.Metrics.Environment == "" {
		errs = append(errs, errors.New("no metrics environment specified"))
	}

	return errs
}
