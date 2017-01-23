package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
)

func LoadCertificate(certPath, keyPath, password string) (tls.Certificate, error) {
	if password == "" {
		return tls.LoadX509KeyPair(certPath, keyPath)
	}

	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}

	encryptedKeyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}

	block, _ := pem.Decode(encryptedKeyBytes)
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
	keyBytes := pem.EncodeToMemory(pemBlock)

	return tls.X509KeyPair(certBytes, keyBytes)
}

func LoadCertificatePool(certFiles ...string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, certFileName := range certFiles {
		bs, err := ioutil.ReadFile(certFileName)
		if err != nil {
			return certPool, err
		}

		ok := certPool.AppendCertsFromPEM(bs)
		if !ok {
			return certPool, fmt.Errorf("failed to append client certs from pem: %s", err.Error())
		}
	}

	return certPool, nil
}
