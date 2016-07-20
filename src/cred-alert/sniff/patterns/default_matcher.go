package patterns

const generalPattern = `(?i)("|')*[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']*\s*(=|:|\s|:=|=>)\s*["'][A-Za-z0-9.$+=&\/_\\-]{12,}("|')`
const awsAccessKeyIDPattern = `AKIA[A-Z0-9]{16}`
const awsSecretAccessKeyPattern = `(?i)("|')?(aws)?_?(secret)?_?(access)?_?(key)("|')?\s*(:|=>|=)\s*("|')?[A-Za-z0-9/\+=]{40}("|')?`
const awsAccountIDPattern = `(?i)("|')?(aws)?_?(account)_?(id)?("|')?\s*(:|=>|=)\s*("|')?[0-9]{4}\-?[0-9]{4}\-?[0-9]{4}("|')?`
const cryptMD5Pattern = `\$1\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{22}`
const cryptSHA256Pattern = `\$5\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{43}`
const cryptSHA512Pattern = `\$6\$[a-zA-Z0-9./]{16}\$[a-zA-Z0-9./]{86}`
const rsaPrivateKeyHeaderPattern = `-----BEGIN RSA PRIVATE KEY-----`

const bashStringInterpolationPattern = `["]\$`
const fakePattern = `(?i)fake`
const examplePattern = `(?i)example`

func DefaultMatcher() Matcher {
	return NewMatcher(
		[]string{
			generalPattern,
			awsAccessKeyIDPattern,
			awsSecretAccessKeyPattern,
			awsAccountIDPattern,
			cryptMD5Pattern,
			cryptSHA256Pattern,
			cryptSHA512Pattern,
			rsaPrivateKeyHeaderPattern,
		},
		[]string{
			bashStringInterpolationPattern,
			fakePattern,
			examplePattern,
		},
	)
}
