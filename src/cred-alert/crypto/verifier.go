package crypto

//go:generate counterfeiter . Verifier

type Verifier interface {
	Verify([]byte, []byte) error
}
