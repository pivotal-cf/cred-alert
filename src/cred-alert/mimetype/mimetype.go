package mimetype

//go:generate counterfeiter . Mimetype

type Mimetype interface {
	TypeByBuffer([]byte) (string, error)
}
