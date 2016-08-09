package inflator

import (
	"io/ioutil"
	"os"
)

//go:generate counterfeiter . ScratchSpace

type ScratchSpace interface {
	Make() (string, error)
}

func NewScratch() *scratch {
	return &scratch{}
}

type scratch struct{}

func (s *scratch) Make() (string, error) {
	return ioutil.TempDir("", "inflator-scratch")
}

func NewDeterministicScratch(path string) *detScratch {
	return &detScratch{
		path: path,
	}
}

type detScratch struct {
	path string
}

func (s *detScratch) Make() (string, error) {
	err := os.MkdirAll(s.path, 0755)
	if err != nil {
		return "", err
	}

	return s.path, nil
}
