package git_test

var sampleDiff = `diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests.txt
+++ b/spec/integration/git-secrets-pattern-tests.txt
@@ -28,7 +28,7 @@ private_key = "should_match" # COMMENT: comments shouldn't have an effect
 private_key '$should_match'
 
 ## Suspicious Variable Names
-some_secret: "should-match"
+some_secret: "should_match"
 hard_coded_salt: "should_match"
 private_key: "should_match"
 some_password: "should_match"
@@ -36,6 +36,14 @@ some_password: "should_match"
 ## Special Characters
 private_key: ".$+=&/\\-_should_match"
 
+# Case
+AWS_ACCESS_KEY_ID: AKIAIOSFODNN7DSOTPWI # should_match
+AWS_SECRET_ACCESS_KEY: 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 # should_match
+AWS_ACCOUNT_ID: 1234-1234-1234 # should_match
+SOME_SECRET: "should_match"
+HARD_CODED_SALT: "should_match"
+PRIVATE_KEY: "should_match"
+SOME_PASSWORD: "should_match"
 
 # Should Not Match
 
@@ -67,6 +75,8 @@ private_key: "should not match"
 private_key: "too-short" # should_not_match
 private_key: "fake_should_not_match"
 private_key: "example_should_not_match"
+private_key: "FaKe_should_not_match"
+private_key: "ExAmPlE_should_not_match"
 
 ## Misc. Exclusions
 ### 20 digit number
diff --git a/spec/integration/git-secrets-pattern-tests.txt b/spec/integration/git-secrets-pattern-tests2.txt
index 940393e..fa5a232 100644
--- a/spec/integration/git-secrets-pattern-tests2.txt
+++ b/spec/integration/git-secrets-pattern-tests2.txt
@@ -28,7 +28,7 @@ private_key = "should_match" # COMMENT: comments shouldn't have an effect
 private_key '$should_match'
 
 ## Suspicious Variable Names
-some_secret: "should-match"
+some_secret: "should_match"
 hard_coded_salt: "should_match"
 private_key: "should_match"
 some_password: "should_match"
`
