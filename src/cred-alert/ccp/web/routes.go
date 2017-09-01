package web

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	Organization = "Organization"
	Repository   = "Repository"
)

var Routes = rata.Routes{
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/organizations/:organization", Method: "GET", Name: Organization},
	{Path: "/organizations/:organization/repositories/:repository", Method: "GET", Name: Repository},
}
