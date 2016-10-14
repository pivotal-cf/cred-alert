package migrations

import "github.com/BurntSushi/migration"

var Migrations = []migration.Migrator{
	InitialSchema,
	AddAutoIncrement,
	DropCommitTimestamp,
	AddCredentialsAndScansTable,
	AddRepositoriesAndFetches,
	AddClonedToRepositories,
	AddMatchLocation,
	AddFailedFetchesToRepositories,
	AddStartSHAAndStopSHAToScans,
	AddPrivateToCredentials,
}
