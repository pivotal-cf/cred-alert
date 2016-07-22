package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"cred-alert/sniff/matchers"
)

var _ = Describe("Assignment Matcher", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.Assignment()
	})

	DescribeTable("not matching other assignments",
		func(line string) {
			Expect(matcher.Match(line)).To(BeFalse())
		},
		Entry("not an assignment", "package not_an_assignment"),
		Entry("RHS is too short", "password too-short"),
		Entry("no quotes with equals sign", "password = should_match"),
		Entry("YAML assignment with a GUID", "v5_private_key: 6392b811-01d8-5c72-a68c-6d85f2a4b02b"),
	)

	DescribeTable("matching secret assignments",
		func(line string) {
			Expect(matcher.Match(line)).To(BeTrue())
		},
		Entry("simple assignment with no operator", "password 'should_match'"),
		Entry("simple assignment with colon", "password: 'should_match'"),
		Entry("simple assignment with equals", "password = 'should_match'"),
		Entry("simple assignment with colon equals", "password := 'should_match'"),
		Entry("simple assignment with a rocket", "password => 'should_match'"),
		Entry("simple assignment with no spaces", "password='should_match'"),
		Entry("simple assignment with double quotes", `password = "should_match"`),
		Entry("simple assignment with different variable names (private-key)", "private-key = 'should_match'"),
		Entry("simple assignment with different variable names (private_key)", "private_key = 'should_match'"),
		Entry("simple assignment with different variable names (secret)", "secret = 'should_match'"),
		Entry("simple assignment with different variable names (salt)", "salt = 'should_match'"),
		Entry("simple assignment with a prefixed variable names", "hello_password = 'should_match'"),
		Entry("simple assignment with a strange cased variable names", "PaSSwoRD = 'should_match'"),
		Entry("simple assignment with a comment", `private_key = "should_match" # COMMENT: comments shouldn't have an effect`),
		Entry("simple assignment with strange characters", `password = '.$+=&/\\should_match' # comment`),
		Entry("YAML assignment", "password: should_match"),
	)
})
