package db

type Branch struct {
	Model

	RepositoryID uint

	Name            string
	CredentialCount uint
}
