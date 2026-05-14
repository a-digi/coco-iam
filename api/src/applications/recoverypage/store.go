// Package recoverypage owns the per-application password recovery
// flow (request + reset). Custom HTML templates are no longer
// supported — the public recovery pages are rendered by the React
// app using the login-template settings.
package recoverypage

import (
	"database/sql"
	"errors"

	"github.com/a-digi/coco-orm/orm"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbm *orm.DatabaseManager) *Store {
	return &Store{db: dbm.Connector.DB}
}

var ErrNotFound = errors.New("recoverypage: not found")
