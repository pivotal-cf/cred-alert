package queue

import "github.com/satori/go.uuid"

//go:generate counterfeiter . UUIDGenerator

type UUIDGenerator interface {
	Generate() string
}

type uuidGenerator struct{}

func NewGenerator() *uuidGenerator {
	return &uuidGenerator{}
}

func (u *uuidGenerator) Generate() string {
	return uuid.Must(uuid.NewV4()).String()
}
