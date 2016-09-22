package matchers_test

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"cred-alert/scanners"
	"cred-alert/sniff/matchers"
)

var _ = Describe("Assignment Matcher", func() {
	var matcher matchers.Matcher

	BeforeEach(func() {
		matcher = matchers.Assignment()
	})

	DescribeTable("not matching other assignments",
		func(content string, name ...string) {
			fileName := "a-file.txt"
			if len(name) > 0 {
				fileName = name[0]
			}

			line := &scanners.Line{
				Content:    []byte(strings.ToUpper(content)),
				Path:       filepath.Join("this", "is", "a", "path", "to", fileName),
				LineNumber: 42,
			}

			matched, _, _ := matcher.Match(line)
			Expect(matched).To(BeFalse())
		},
		Entry("not an assignment", "package not_an_assignment"),
		Entry("RHS is too short", "password too-short"),
		Entry("no quotes with equals sign", "password = should_match"),
		Entry("Text with placeholder", "suspect_password: placeholder-for-anything"),
		Entry("YAML assignment with a GUID", "v5_private_key: 6392b811-01d8-5c72-a68c-6d85f2a4b02b", "manifest.yml"),
		Entry("YAML assignment that is 10 characters", "password: should_mat", "manifest.yml"),
		Entry("YAML assignment with placeholder", "suspect_password: ((placeholder-for-anything))", "manifest.yml"),
		Entry("YAML assignment with placeholder with no whitespace", "suspect_password:((placeholder-for-anything))", "manifest.yml"),
		Entry("YAML assignment with fly placeholder", "suspect_password: {{placeholder-for-anything}}", "manifest.yml"),
	)

	DescribeTable("matching secret assignments",
		func(content string, expectedStart, expectedEnd int, maybeFilename ...string) {
			fileName := "a-file.txt"
			if len(maybeFilename) > 0 {
				fileName = maybeFilename[0]
			}

			line := &scanners.Line{
				Content:    []byte(strings.ToUpper(content)),
				Path:       filepath.Join("this", "is", "a", "path", "to", fileName),
				LineNumber: 42,
			}

			matched, start, end := matcher.Match(line)
			Expect(matched).To(BeTrue())
			Expect(start).To(Equal(expectedStart))
			Expect(end).To(Equal(expectedEnd))
		},
		Entry("simple assignment with no operator", "password 'should_match'", 0, 23),
		Entry("simple assignment with a dash on the RHS", "password = 'should-match'", 0, 25),
		Entry("simple assignment with colon", "password: 'should_match'", 0, 24),
		Entry("simple assignment with equals", "password = 'should_match'", 0, 25),
		Entry("simple assignment with colon equals", "password := 'should_match'", 0, 26),
		Entry("simple assignment with a rocket", "password => 'should_match'", 0, 26),
		Entry("simple assignment with no spaces", "password='should_match'", 0, 23),
		Entry("simple assignment with double quotes", `password = "should_match"`, 0, 25),
		Entry("simple assignment with different variable names (private-key)", "private-key = 'should_match'", 0, 28),
		Entry("simple assignment with different variable names (private_key)", "private_key = 'should_match'", 0, 28),
		Entry("simple assignment with different variable names (secret)", "secret = 'should_match'", 0, 23),
		Entry("simple assignment with different variable names (salt)", "salt = 'should_match'", 0, 21),
		Entry("simple assignment with a prefixed variable names", "hello_password = 'should_match'", 6, 31),
		Entry("simple assignment with a strange cased variable names", "PaSSwoRD = 'should_match'", 0, 25),
		Entry("simple assignment with a comment", `private_key = "should_match" # COMMENT: comments shouldn't have an effect`, 0, 28),
		Entry("simple assignment with strange characters", `password = '.$+=&/\\should_match' # comment`, 0, 33),
		Entry("YAML assignment", "password: should_match", 0, 22, "manifest.yml"),
		Entry("YAML assignment with a silly extension", "password: should_match", 0, 22, "manifest.yaml"),
		Entry("YAML assignment with mismatched placeholder values", "password: {(should_match)}", 0, 26, "manifest.yaml"),
		Entry("YAML assignment with whitespace around the placeholder values", "password: (( should_match ))", 0, 28, "manifest.yaml"),
		Entry("YAML assignment with non-placeholder", "suspect_password: placeholder-for-anything", 8, 42, "manifest.yml"),
		Entry("YAML assignment with non-placeholder", "secret_password: this-is-a-placeholder", 7, 38, "manifest.yml"),
	)
})
