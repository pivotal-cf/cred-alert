package db

type Commit struct {
	Model
	Owner      string
	Repository string
	SHA        string
}
