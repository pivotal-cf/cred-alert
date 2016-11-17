package crypto

//go:generate counterfeiter . Signer

type Signer interface {
	Sign([]byte) ([]byte, error)
}
