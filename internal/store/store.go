package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reqiewu/order-bot/internal/catalog"
	"go.etcd.io/bbolt"
)

var (
	bucketMeta  = []byte("meta")
	bucketSlots = []byte("watch_slots")
)

// Store — Bbolt для whitelist/настроек (токены — позже encrypted).
type Store struct {
	db *bbolt.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, err
	}
	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, b := range [][]byte{bucketMeta, bucketSlots} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) PutSlot(slot catalog.WatchSlot) error {
	if !slot.Valid() {
		return fmt.Errorf("store: empty collection")
	}
	if slot.ID == "" {
		slot.ID = slot.Collection + "|" + slot.Model + "|" + slot.Backdrop
	}
	raw, err := json.Marshal(slot)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketSlots).Put([]byte(slot.ID), raw)
	})
}

func (s *Store) ListSlots() ([]catalog.WatchSlot, error) {
	var out []catalog.WatchSlot
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketSlots).ForEach(func(_, v []byte) error {
			var slot catalog.WatchSlot
			if err := json.Unmarshal(v, &slot); err != nil {
				return err
			}
			out = append(out, slot)
			return nil
		})
	})
	return out, err
}

func (s *Store) SetMeta(key, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketMeta).Put([]byte(key), []byte(value))
	})
}

func (s *Store) GetMeta(key string) (string, error) {
	var v []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		v = append([]byte(nil), tx.Bucket(bucketMeta).Get([]byte(key))...)
		return nil
	})
	return string(v), err
}
