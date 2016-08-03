package fixtures

var Credentials = `
# Git secrets extra patterns test file

# Should Match

# AWS Credentials

aws_access_key_id: AKIAIOSFODNN7DSOTPWI # should_match
aws_secret_access_key: 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 # should_match

## Operators
password 'should_match'
password: 'should_match'
password = 'should_match'
password := 'should_match'
password => 'should_match'

## Quotes
password "should_match"

## Syntax
private_key 'should_match'
private_key: 'should_match'
var privateKey = 'should_match'
private static String PrivateKey = 'should_match'
  private_key = "should_match"
private_key = "should_match" # COMMENT: comments shouldn't have an effect
private_key '$should_match'

## Suspicious Variable Names
some_secret: "should_match"
hard_coded_salt: "should_match"
private_key: "should_match"
some_password: "should_match"

## Special Characters
private_key: ".$+=&/\\-_should_match"

# Case
AWS_ACCESS_KEY_ID: AKIAIOSFODNN7DSOTPWI # should_match
AWS_SECRET_ACCESS_KEY: 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 # should_match
SOME_SECRET: "should_match"
HARD_CODED_SALT: "should_match"
PRIVATE_KEY: "should_match"
SOME_PASSWORD: "should_match"

# UNIX passwords

$6$0./3456789abcdef$1./45678901234567890123456789012345678901234567890123456789012345678900123456789abcdef # should_match
$5$ABCDEF0./3456789$1./4567890123456789012345678900123456789abc # should_match
$1$ABCDEF0./3456789$1./47/8900123456789abc # should_match

# RSA Key

  -----BEGIN RSA PRIVATE KEY----- # should_match

# Should Not Match

## Syntax Exclusions
varible_with_private_in_it = "should_not_match"
variable-with-private-key-in-it: "should_not_match"
variableWithPrivateKeyInIt = "should_not_match"
private_key = should_not_match
private_key="$bash_variable_should_not_match"
private_key=$bash_variable_should_not_match

## Variable Name Exclusions
variable_with_secret_in_it: "should_not_match"
variabe_with_salt_in_it: "should_not_match"
variable_with_private_key_in_it: "should_not_match"
variable_with_password_in_it: "should_not_match"

## Special Character Exclusions
### Bash
private_key: "${should_not_match}"
### Ruby
private_key: "{{should_not_match}}"
### Misc.
private_key: "%%%_should_not_match"

## Content Exclusions
private_key: "should not match"
private_key: "too-short" # should_not_match
private_key: "fake_should_not_match"
private_key: "example_should_not_match"
private_key: "FaKe_should_not_match"
private_key: "ExAmPlE_should_not_match"

## UUID
v1_private_key: 416a0dc0-4f63-11e6-9abc-0000000341c5 # should_not_match
v2_private_key: 416a0dc0-4f63-21e6-9abc-0000000341c5 # should_not_match
v3_private_key: 06418540-2fbb-3cf7-bc00-521b1ffb6074 # should_not_match
v4_private_key: 3A84A61B-32EB-4889-B8DA-46ACCC8C8813 # should_not_match
v4_lower_private_key: "3a84a61b-32eb-4889-b8da-46accc8c8813" # should_not_match
v5_private_key: 6392b811-01d8-5c72-a68c-6d85f2a4b02b # should_not_match

## Misc. Exclusions
### 20 digit number
'uuid' => '12324234234234234234' # should_not_match
GO15VENDOREXPERIMENT # should_not_match
### Java long
private static final long serialVersionUID = -9999999999999999999L; # should_not_match
`
