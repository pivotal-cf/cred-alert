package rolodex_test

import (
	"rolodex"

	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamRepository", func() {
	var (
		teamsPath string
	)

	Describe("GetOwners", func() {
		Context("when the directory does not exist", func() {
			It("returns no teams", func() {
				teamRepo := rolodex.NewTeamRepository("/some/garbage")

				teams, err := teamRepo.GetOwners(rolodex.Repository{
					Owner: "cloudfoundry",
					Name:  "bosh",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(teams).To(BeEmpty())
			})
		})

		Context("when the directory exists", func() {
			var writeFile = func(filename, data string) {
				filePath := filepath.Join(teamsPath, filename)
				err := ioutil.WriteFile(filePath, []byte(data), 0600)
				Expect(err).NotTo(HaveOccurred())
			}

			BeforeEach(func() {
				var err error
				teamsPath, err = ioutil.TempDir("", "teams")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.RemoveAll(teamsPath)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the directory is empty", func() {
				It("returns no teams", func() {
					teamRepo := rolodex.NewTeamRepository(teamsPath)

					teams, err := teamRepo.GetOwners(rolodex.Repository{
						Owner: "cloudfoundry",
						Name:  "bosh",
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(teams).To(BeEmpty())
				})
			})

			Context("when there are files in the directory", func() {
				It("returns matching teams", func() {
					writeFile("bosh.yml", `---
name: bosh

repositories:
- cloudfoundry/bosh
- cloudfoundry/bosh-agent
`)

					writeFile("capi.yml", `---
name: capi

repositories:
- cloudfoundry/capi
- cloudfoundry/capri-sun

contact:
  slack:
    team: cloudfoundry
    channel: capi
`)

					teamRepo := rolodex.NewTeamRepository(teamsPath)

					teams, err := teamRepo.GetOwners(rolodex.Repository{
						Owner: "cloudfoundry",
						Name:  "capi",
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(teams).To(ConsistOf(
						rolodex.Team{
							Name: "capi",
							SlackChannel: rolodex.SlackChannel{
								Team: "cloudfoundry",
								Name: "capi",
							},
						},
					))
				})

				It("returns multiple matching teams", func() {
					writeFile("bosh-1.yml", `---
name: bosh-1

repositories:
- cloudfoundry/bosh
`)

					writeFile("bosh-2.yml", `---
name: bosh-2

repositories:
- cloudfoundry/bosh
`)

					teamRepo := rolodex.NewTeamRepository(teamsPath)

					teams, err := teamRepo.GetOwners(rolodex.Repository{
						Owner: "cloudfoundry",
						Name:  "bosh",
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(teams).To(ConsistOf(
						rolodex.Team{Name: "bosh-1"},
						rolodex.Team{Name: "bosh-2"},
					))
				})

				It("only returns teams once if repositories are repeated", func() {
					writeFile("bosh.yml", `---
name: bosh

repositories:
- cloudfoundry/bosh
- cloudfoundry/bosh
`)

					teamRepo := rolodex.NewTeamRepository(teamsPath)

					teams, err := teamRepo.GetOwners(rolodex.Repository{
						Owner: "cloudfoundry",
						Name:  "bosh",
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(teams).To(ConsistOf(
						rolodex.Team{Name: "bosh"},
					))
				})
			})
		})
	})
})
