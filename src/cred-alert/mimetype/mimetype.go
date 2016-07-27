package mimetype

//go:generate counterfeiter . Decoder

type Decoder interface {
	TypeByBuffer([]byte) (string, error)
}
