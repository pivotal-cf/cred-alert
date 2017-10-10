package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func LoadCertificate(cert, key, password string) (tls.Certificate, error) {
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

func LoadCertificatePool(certs ...string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, cert := range certs {
		ok := certPool.AppendCertsFromPEM([]byte(cert))
		if !ok {
			return certPool, fmt.Errorf("failed to append client certs from pem")
		}
	}

	return certPool, nil
}
