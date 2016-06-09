package entropy

import "github.com/nbutton23/zxcvbn-go"

func IsPasswordSuspect(candidate string) bool {
	match := zxcvbn.PasswordStrength(candidate, []string{})

	entropyPerChar := match.Entropy / float64(len(candidate))

	return entropyPerChar > 3.7 // magic magic magic
}
