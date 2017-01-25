package rolodex

import (
	"errors"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

func LoadRolodexConfig(configPath string) (*RolodexConfig, error) {
	bs, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	c := &RolodexConfig{}
	err = yaml.Unmarshal(bs, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type RolodexConfig struct {
	Metrics struct {
		SentryDSN     string `yaml:"sentry_dsn"`
		DatadogAPIKey string `yaml:"datadog_api_key"`
		Environment   string `yaml:"environment"`
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

func (c *RolodexConfig) Validate() []error {
	var errs []error

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
