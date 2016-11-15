package web

import "github.com/tedsuo/rata"

const (
	Index = "Index"
)

var Routes = rata.Routes{
	{Path: "/", Method: "GET", Name: Index},
}
