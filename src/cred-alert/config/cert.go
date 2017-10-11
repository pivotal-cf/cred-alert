package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
)

// LoadCertificateFromFiles returns a TLS certificate from the provided paths to
// certificate and key, decoding with the password if provided
func LoadCertificateFromFiles(certPath, keyPath, password string) (tls.Certificate, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}

	return LoadCertificate(certBytes, keyBytes, password)
}

// LoadCertificate returns a TLS certificate from the provided certificate and
// key, decoding with the password if provided
func LoadCertificate(cert []byte, key []byte, password string) (tls.Certificate, error) {
	certBytes := []byte(cert)
	keyBytes := []byte(key)

	if password == "" {
		return tls.X509KeyPair(certBytes, keyBytes)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return tls.Certificate{}, errors.New("error decoding PEM in key file")
	}

	decryptedKeyBytes, err := x509.DecryptPEMBlock(block, []byte(password))
	if err != nil {
		return tls.Certificate{}, err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY", // go don't care
		Bytes: decryptedKeyBytes,
	}
	encodedKeyBytes := pem.EncodeToMemory(pemBlock)

	return tls.X509KeyPair(certBytes, encodedKeyBytes)
}

func LoadCertificatePool(certs ...[]byte) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, cert := range certs {
		ok := certPool.AppendCertsFromPEM([]byte(cert))
		if !ok {
			return certPool, fmt.Errorf("failed to append client certs from pem")
		}
	}

	return certPool, nil
}

func LoadCertificatePoolFromFiles(certPaths ...string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, certPath := range certPaths {
		bs, err := ioutil.ReadFile(certPath)
		if err != nil {
			return certPool, err
		}

		ok := certPool.AppendCertsFromPEM(bs)

		if !ok {
			return certPool, fmt.Errorf("failed to append client certs from pem")
		}
	}

	return certPool, nil
}
