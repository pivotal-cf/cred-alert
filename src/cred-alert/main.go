package main

import (
	"bufio"
	"fmt"
	"os"

	"cred-alert/patterns"
)

func main() {
	matcher := patterns.DefaultMatcher()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		found := matcher.Match(line)

		if found {
			fmt.Println("Matches pattern: ", line)
		}

		// found = entropy.IsPasswordSuspect(line)
		// if found {
		// 	fmt.Println("Matches entropy: ", line)
		// }
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
