// Package store provides event trace storage using BadgerDB.
package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// BadgerStore provides raw event trace storage with configurable retention.
type BadgerStore struct {
	db        *badger.DB
	retention time.Duration
	logger    *slog.Logger
}

// NewBadgerStore opens or creates a BadgerDB database.
func NewBadgerStore(dataDir string, retention time.Duration, logger *slog.Logger) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dataDir).
		WithLogger(nil). // Use our own logger
		WithValueLogFileSize(256 * 1024 * 1024) // 256MB value log files

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("opening BadgerDB at %s: %w", dataDir, err)
	}

	store := &BadgerStore{
		db:        db,
		retention: retention,
		logger:    logger,
	}

	// Start background GC
	go store.gcLoop()

	return store, nil
}

// StoreTrace stores a raw HTTP trace event.
func (s *BadgerStore) StoreTrace(traceID string, timestamp time.Time, data interface{}) error {
	// Key: timestamp (nanoseconds) + traceID for ordered iteration
	key := fmt.Sprintf("%020d:%s", timestamp.UnixNano(), traceID)

	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling trace: %w", err)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), value).
			WithTTL(s.retention)
		return txn.SetEntry(entry)
	})
}

// QueryTraces retrieves traces within a time range, optionally filtered by service.
func (s *BadgerStore) QueryTraces(start, end time.Time, service string, limit int) ([]json.RawMessage, error) {
	startKey := fmt.Sprintf("%020d:", start.UnixNano())
	endKey := fmt.Sprintf("%020d:", end.UnixNano())

	var results []json.RawMessage

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Seek([]byte(startKey)); it.Valid(); it.Next() {
			if count >= limit {
				break
			}

			item := it.Item()
			key := string(item.Key())

			// Stop if past the end time
			if key > endKey {
				break
			}

			err := item.Value(func(val []byte) error {
				// If service filter is specified, check it
				if service != "" {
					// Quick check — full filtering done by caller
					var trace map[string]interface{}
					if err := json.Unmarshal(val, &trace); err == nil {
						if svc, ok := trace["service"].(string); ok && svc != service {
							return nil
						}
					}
				}

				result := make(json.RawMessage, len(val))
				copy(result, val)
				results = append(results, result)
				count++
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return results, err
}

// gcLoop runs periodic garbage collection on BadgerDB.
func (s *BadgerStore) gcLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		for {
			err := s.db.RunValueLogGC(0.5)
			if err != nil {
				break // No more GC needed
			}
		}
		s.logger.Debug("BadgerDB GC completed")
	}
}

// Close cleanly shuts down the database.
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// DiskSize returns the current disk usage of the database.
func (s *BadgerStore) DiskSize() (int64, error) {
	lsm, vlog := s.db.Size()
	return lsm + vlog, nil
}
