// Package db provides helpers to connect to test databases.  It has no
// internal dependencies on aurora and so should be able to be imported by
// any aurora package.
package db

import (
	"fmt"
	"log"
	"testing"

	// pq enables postgres support
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	db "github.com/HashCash-Consultants/go/support/db/dbtest"
)

var (
	auroraDB     *db.DB
	coreDB        *db.DB
	coreDBConn    *sqlx.DB
	auroraDBConn *sqlx.DB
)

func auroraPostgres(t *testing.T) *db.DB {
	if auroraDB != nil {
		return auroraDB
	}
	auroraDB = db.Postgres(t)
	return auroraDB
}

// TODO, remove refs to internal core db, need to remove scenario tests which require this
// to seed core db.
func corePostgres(t *testing.T) *db.DB {
	if coreDB != nil {
		return coreDB
	}
	coreDB = db.Postgres(t)
	return coreDB
}

func Aurora(t *testing.T) *sqlx.DB {
	if auroraDBConn != nil {
		return auroraDBConn
	}

	auroraDBConn = auroraPostgres(t).Open()
	return auroraDBConn
}

func AuroraURL() string {
	if auroraDB == nil {
		log.Panic(fmt.Errorf("Aurora not initialized"))
	}
	return auroraDB.DSN
}

func AuroraROURL() string {
	if auroraDB == nil {
		log.Panic(fmt.Errorf("Aurora not initialized"))
	}
	return auroraDB.RO_DSN
}

// TODO, remove refs to core db, need to remove scenario tests which require this
// to seed core db.
func HcnetCore(t *testing.T) *sqlx.DB {
	if coreDBConn != nil {
		return coreDBConn
	}
	coreDBConn = corePostgres(t).Open()
	return coreDBConn
}

// TODO, remove refs to core db, need to remove scenario tests which require this
// to seed core db.
func HcnetCoreURL() string {
	if coreDB == nil {
		log.Panic(fmt.Errorf("HcnetCore not initialized"))
	}
	return coreDB.DSN
}
