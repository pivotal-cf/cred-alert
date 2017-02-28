package config

func allBlankOrAllSet(xs ...string) bool {
	var blanks int
	for i := range xs {
		if xs[i] == "" {
			blanks++
		}
	}

	return blanks == len(xs) || blanks == 0
}
