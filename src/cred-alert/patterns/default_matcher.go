package patterns

func DefaultMatcher() *Matcher {
	return NewMatcher(
		[]string{
			`AKIA[A-Z0-9]{16}`,
			`(?i)("|')?(aws)?_?(secret)?_?(access)?_?(key)("|')?\s*(:|=>|=)\s*("|')?[A-Za-z0-9/\+=]{40}("|')?`,
			`(?i)("|')?(aws)?_?(account)_?(id)?("|')?\s*(:|=>|=)\s*("|')?[0-9]{4}\-?[0-9]{4}\-?[0-9]{4}("|')?`,
			`(?i)("|')*[A-Za-z0-9_-]*(secret|private[-_]?key|password|salt)["']*\s*(=|:|\s|:=|=>)\s*["'][A-Za-z0-9.$+=&\/_\\-]{12,}("|')`,
		},
		[]string{
			`["]\$`,
			`(?i)fake`,
			`(?i)example`,
		},
	)
}
