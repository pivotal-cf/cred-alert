package scanners

type Line struct {
	Path       string
	LineNumber int
	Content    []byte
}

type Violation struct {
	Line Line

	Start int
	End   int
}

func (v Violation) Credential() string {
	return string((v.Line.Content)[v.Start:v.End])
}
