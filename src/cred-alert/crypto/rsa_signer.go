package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
)

type rsaSigner struct {
	priv *rsa.PrivateKey
}

func NewRSASigner(priv *rsa.PrivateKey) Signer {
	return &rsaSigner{
		priv: priv,
	}
}

func (r *rsaSigner) Sign(msg []byte) ([]byte, error) {
	hash := sha256.Sum256(msg)
	return rsa.SignPKCS1v15(rand.Reader, r.priv, crypto.SHA256, hash[:])
}
