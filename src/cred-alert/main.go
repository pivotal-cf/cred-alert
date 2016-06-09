package main

import (
	"fmt"
	"os"

	"cred-alert/entropy"
)

func main() {
	fmt.Println("password?", entropy.IsPasswordSuspect(os.Args[1]))
}
