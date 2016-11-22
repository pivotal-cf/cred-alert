package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

func ReadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	der, err := parsePEMintoDER(path)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}

	rsaKey := key.(*rsa.PublicKey)

	return rsaKey, nil
}

func ReadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	der, err := parsePEMintoDER(path)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, err
	}

	return key, nil
}

func parsePEMintoDER(path string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}

	der, _ := pem.Decode(bytes)
	if der == nil {
		return []byte{}, errors.New("Unable to parse PEM")
	}

	return der.Bytes, nil
}
