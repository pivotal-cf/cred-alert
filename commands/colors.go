package commands

import (
	"fmt"
)

func SimpleColorFunc(offset int) func(string) string {
	return func(text string) string {
		return fmt.Sprintf("\033[1;%dm%s\033[0m", 30 + offset, text)
	}
}

var red = SimpleColorFunc(1)
var yellow = SimpleColorFunc(3)
var green = SimpleColorFunc(2)
