package models

import "time"

type Model struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct {
	Model
	Org       string
	Name      string
	DiffScans []DiffScan
}

type DiffScan struct {
	Model
	Repo         Repo
	RepoID       uint
	FromCommit   Commit `gorm:"ForeignKey:FromCommitID"`
	FromCommitID uint
	ToCommit     Commit `gorm:"ForeignKey:ToCommitID"`
	ToCommitID   uint
}

type Commit struct {
	Model
	SHA       string
	Timestamp time.Time
}
