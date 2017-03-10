package db_test

import (
	"cred-alert/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BranchRepository", func() {

	var (
		branchRepository     db.BranchRepository
		repositoryRepository db.RepositoryRepository

		repository db.Repository
	)

	BeforeEach(func() {
		database, err := dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		branchRepository = db.NewBranchRepository(database)

		repositoryRepository = db.NewRepositoryRepository(database)

		repository = db.Repository{
			Owner:   "my-special-owner",
			Name:    "my-special-name",
		}

		err = repositoryRepository.Create(&repository)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("UpdateBranches", func() {
		It("creates and gets branches", func() {
			err := branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 42,
				},
				{
					Name:            "branch-2",
					CredentialCount: 56,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			branches, err := branchRepository.GetBranches(repository)
			Expect(err).NotTo(HaveOccurred())

			Expect(branches).To(HaveLen(2))
			Expect(branches[0].RepositoryID).To(Equal(repository.ID))
			Expect(branches[0].Name).To(Equal("branch-1"))
			Expect(branches[0].CredentialCount).To(Equal(uint(42)))
		})

		It("creates branches atomically", func() {
			err := branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name: "branch-1",
				},
				{
					Name: "branch-1",
				},
			})
			Expect(err).To(HaveOccurred())

			branches, err := branchRepository.GetBranches(repository)
			Expect(err).NotTo(HaveOccurred())

			Expect(branches).To(HaveLen(0))
		})

		It("completely replaces all branches for the repository", func() {
			err := branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 42,
				},
				{
					Name:            "branch-2",
					CredentialCount: 8,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			err = branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 56,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			branches, err := branchRepository.GetBranches(repository)
			Expect(err).NotTo(HaveOccurred())

			Expect(branches).To(HaveLen(1))
			Expect(branches[0].Name).To(Equal("branch-1"))
			Expect(branches[0].CredentialCount).To(Equal(uint(56)))
		})

		It("allows branches that have the same name (case-insensitively)", func() {
			err := branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name:            "BRANCH",
					CredentialCount: 42,
				},
				{
					Name:            "branch",
					CredentialCount: 8,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetCredentialCountByOwner", func() {
		var otherRepository db.Repository

		BeforeEach(func() {
			otherRepository = db.Repository{
				Owner:   "my-different-owner",
				Name:    "my-special-name",
			}

			err := repositoryRepository.Create(&otherRepository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sums up all of the credential counts per organization", func() {
			err := branchRepository.UpdateBranches(repository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 42,
				},
				{
					Name:            "branch-2",
					CredentialCount: 56,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			err = branchRepository.UpdateBranches(otherRepository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 3,
				},
				{
					Name:            "branch-2",
					CredentialCount: 97,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			report, err := branchRepository.GetCredentialCountByOwner()
			Expect(err).NotTo(HaveOccurred())

			Expect(report).To(Equal([]db.OwnerCredentialCount{
				{
					Owner:           "my-different-owner",
					CredentialCount: 100,
				},
				{
					Owner:           "my-special-owner",
					CredentialCount: 98,
				},
			}))
		})
	})

	Describe("GetCredentialCountForOwner", func() {
		var (
			yetAnotherRepository db.Repository
			otherRepository      db.Repository
		)

		BeforeEach(func() {
			otherRepository = db.Repository{
				Owner:   "my-different-owner",
				Name:    "my-special-name",
			}

			err := repositoryRepository.Create(&otherRepository)
			Expect(err).NotTo(HaveOccurred())

			yetAnotherRepository = db.Repository{
				Owner:   "my-different-owner",
				Name:    "my-other-name",
			}

			err = repositoryRepository.Create(&yetAnotherRepository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sums up all of the credential counts per repository", func() {
			err := branchRepository.UpdateBranches(otherRepository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 42,
				},
				{
					Name:            "branch-2",
					CredentialCount: 56,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			err = branchRepository.UpdateBranches(yetAnotherRepository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 7,
				},
				{
					Name:            "branch-2",
					CredentialCount: 35,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			report, err := branchRepository.GetCredentialCountForOwner("my-different-owner")
			Expect(err).NotTo(HaveOccurred())

			Expect(report).To(Equal([]db.RepositoryCredentialCount{
				{
					Owner:           "my-different-owner",
					Name:            "my-other-name",
					CredentialCount: 42,
				},
				{
					Owner:           "my-different-owner",
					Name:            "my-special-name",
					CredentialCount: 98,
				},
			}))
		})
	})

	Describe("GetCredentialCountForRepo", func() {
		var (
			otherRepository db.Repository
		)

		BeforeEach(func() {
			otherRepository = db.Repository{
				Owner:   "my-different-owner",
				Name:    "my-special-name",
			}

			err := repositoryRepository.Create(&otherRepository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sums up all of the credential counts per branch", func() {
			err := branchRepository.UpdateBranches(otherRepository, []db.Branch{
				{
					Name:            "branch-1",
					CredentialCount: 42,
				},
				{
					Name:            "branch-2",
					CredentialCount: 56,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			report, err := branchRepository.GetCredentialCountForRepo("my-different-owner", "my-special-name")
			Expect(err).NotTo(HaveOccurred())

			Expect(report).To(Equal([]db.BranchCredentialCount{
				{
					Owner:           "my-different-owner",
					Name:            "my-special-name",
					Branch:          "branch-1",
					CredentialCount: 42,
				},
				{
					Owner:           "my-different-owner",
					Name:            "my-special-name",
					Branch:          "branch-2",
					CredentialCount: 56,
				},
			}))
		})
	})
})
