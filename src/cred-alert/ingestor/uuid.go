package ingestor

import "github.com/satori/go.uuid"

type uuidGenerator struct{}

func NewGenerator() *uuidGenerator {
	return &uuidGenerator{}
}

func (u *uuidGenerator) Generate() string {
	return uuid.NewV4().String()
}
