package database

import (
	"database/sql"
)

var DB *sql.DB

func InitDB(dataSourceName string) error {
	var err error
	DB, err = OpenSQLite(dataSourceName)
	if err != nil {
		return err
	}
	return RunMigrations(DB, V1Migrations)
}
