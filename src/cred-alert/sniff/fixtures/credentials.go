package fixtures

var Credentials = `
# Git secrets extra patterns test file

# Should Match

# AWS Credentials

aws_access_key_id: AKIAIOSFODNN7DSOTPWI # should_match
aws_secret_access_key: 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 # should_match

# UNIX passwords

$6$0./3456789abcdef$1./45678901234567890123456789012345678901234567890123456789012345678900123456789abcdef # should_match
$5$ABCDEF0./3456789$1./4567890123456789012345678900123456789abc # should_match
$1$ABCDEF0./3456789$1./47/8900123456789abc # should_match

# RSA Key

  -----BEGIN RSA PRIVATE KEY----- # should_match

`
