package config

import (
	"cred-alert/cmdflag"
	"errors"

	yaml "gopkg.in/yaml.v2"
)

func LoadAPIConfig(bs []byte) (*APIConfig, error) {
	c := &APIConfig{}
	err := yaml.Unmarshal(bs, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type APIOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" value-name:"PATH"`
}

type APIConfig struct {
	Metrics struct {
		SentryDSN   string `yaml:"sentry_dsn"`
		Environment string `yaml:"environment"`
	} `yaml:"metrics"`

	MySQL struct {
		Username             string `yaml:"username"`
		Password             string `yaml:"password"`
		Hostname             string `yaml:"hostname"`
		Port                 uint16 `yaml:"port"`
		DBName               string `yaml:"db_name"`
		ServerName           string `yaml:"server_name"`
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
}

func (c *APIConfig) Validate() []error {
	var errs []error

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
		c.MySQL.CACertificatePath,
		c.MySQL.CertificatePath,
		c.MySQL.PrivateKeyPath,
		c.MySQL.ServerName,
	) {
		errs = append(errs, errors.New("all mysql tls options required if any are set"))
	}

	if !allBlankOrAllSet(
		c.Identity.CACertificatePath,
		c.Identity.CertificatePath,
		c.Identity.PrivateKeyPath,
	) {
		errs = append(errs, errors.New("all identity options required if any are set"))
	}

	return nil
}
