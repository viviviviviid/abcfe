package storage

import (
	"fmt"

	log "github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/config"
	"github.com/syndtr/goleveldb/leveldb"
)

type DB struct {
	db *leveldb.DB
}

func InitDB(cfg *config.Config) (*leveldb.DB, error) {
	dbName := fmt.Sprintf("leveldb_%d.db", cfg.Common.Port)
	dbPath := fmt.Sprintf("%s%s", cfg.DB.Path, dbName)

	// Create DB directory if it does not exist
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		log.Error("Failed to open db: ", err)
		return nil, err
	}

	log.Info("Successfully opened db: ", dbPath)
	return db, nil
}

func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}
