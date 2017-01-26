package rolodex

type Repository struct {
	Owner string
	Name  string
}

type SlackChannel struct {
	Team string
	Name string
}

type Team struct {
	Name         string
	SlackChannel SlackChannel
}
