package web

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	Login        = "Login"
	OAuth        = "Oauth"
	Organization = "Organization"
	Repository   = "Repository"
)

var Routes = rata.Routes{
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/login", Method: "GET", Name: Login},
	{Path: "/auth", Method: "GET", Name: OAuth},
	{Path: "/organizations/:organization", Method: "GET", Name: Organization},
	{Path: "/organizations/:organization/repositories/:repository", Method: "GET", Name: Repository},
}
