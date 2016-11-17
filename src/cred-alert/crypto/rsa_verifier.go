package crypto

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
)

type rsaVerifier struct {
	pub *rsa.PublicKey
}

func NewRSAVerifier(pub *rsa.PublicKey) Verifier {
	return &rsaVerifier{
		pub: pub,
	}
}

func (r *rsaVerifier) Verify(msg []byte, sig []byte) error {
	hash := sha256.Sum256(msg)
	return rsa.VerifyPKCS1v15(r.pub, crypto.SHA256, hash[:], sig)
}
