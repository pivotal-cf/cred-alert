package db_test

import (
	"cred-alert/mysqlrunner"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite")
}

var dbRunner mysqlrunner.Runner

var _ = BeforeSuite(func() {
	dbRunner = mysqlrunner.Runner{
		DBName: fmt.Sprintf("testdb_%d", GinkgoParallelNode()),
	}
	dbRunner.Setup()
})

var _ = AfterSuite(func() {
	dbRunner.Teardown()
})
