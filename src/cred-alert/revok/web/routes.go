package web

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	Organization = "Organization"
)

var Routes = rata.Routes{
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/organizations/:organization", Method: "GET", Name: Organization},
}
