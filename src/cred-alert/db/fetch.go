package db

type Fetch struct {
	Model
	Repository   *Repository
	RepositoryID uint

	Path    string
	Changes []byte
}
