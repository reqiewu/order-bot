package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/reqiewu/order-bot/internal/catalog"
	"github.com/reqiewu/order-bot/internal/money"
	"github.com/reqiewu/order-bot/internal/tokencrypto"
	"go.etcd.io/bbolt"
)

const (
	metaMRKTEnc      = "mrkt_token_enc"
	metaPortalsEnc   = "portals_tma_enc"
	metaGetgemsEnc   = "getgems_api_key_enc"
	metaTonnelEnc    = "tonnel_initdata_enc"
	metaPollSec      = "poll_interval_sec"
	metaMinProfit    = "min_profit_nano"
	metaMinSpreadBPS = "min_spread_bps"
)

// SetEncKey задаёт AES-ключ для токенов (32 байта после ParseKey).
func (s *Store) SetEncKey(key []byte) {
	s.encKey = key
}

// DeleteSlot удаляет слот по id.
func (s *Store) DeleteSlot(id string) error {
	if id == "" {
		return fmt.Errorf("store: empty slot id")
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketSlots).Delete([]byte(id))
	})
}

func (s *Store) seal(plain string) (string, error) {
	if len(s.encKey) == 0 {
		return "", fmt.Errorf("store: encryption key not set")
	}
	blob, err := tokencrypto.Seal(s.encKey, []byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

func (s *Store) open(b64 string) (string, error) {
	if b64 == "" {
		return "", nil
	}
	if len(s.encKey) == 0 {
		return "", fmt.Errorf("store: encryption key not set")
	}
	blob, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	plain, err := tokencrypto.Open(s.encKey, blob)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Store) PutMRKTToken(plain string) error {
	enc, err := s.seal(plain)
	if err != nil {
		return err
	}
	return s.SetMeta(metaMRKTEnc, enc)
}

func (s *Store) MRKTToken() (string, error) {
	enc, err := s.GetMeta(metaMRKTEnc)
	if err != nil {
		return "", err
	}
	return s.open(enc)
}

func (s *Store) HasMRKTToken() bool {
	v, _ := s.GetMeta(metaMRKTEnc)
	return v != ""
}

func (s *Store) PutPortalsTMA(plain string) error {
	enc, err := s.seal(plain)
	if err != nil {
		return err
	}
	return s.SetMeta(metaPortalsEnc, enc)
}

func (s *Store) PortalsTMA() (string, error) {
	enc, err := s.GetMeta(metaPortalsEnc)
	if err != nil {
		return "", err
	}
	return s.open(enc)
}

func (s *Store) HasPortalsTMA() bool {
	v, _ := s.GetMeta(metaPortalsEnc)
	return v != ""
}

func (s *Store) PutGetgemsAPIKey(plain string) error {
	enc, err := s.seal(plain)
	if err != nil {
		return err
	}
	return s.SetMeta(metaGetgemsEnc, enc)
}

func (s *Store) GetgemsAPIKey() (string, error) {
	enc, err := s.GetMeta(metaGetgemsEnc)
	if err != nil {
		return "", err
	}
	return s.open(enc)
}

func (s *Store) HasGetgemsAPIKey() bool {
	v, _ := s.GetMeta(metaGetgemsEnc)
	return v != ""
}

func (s *Store) PutTonnelInitData(plain string) error {
	enc, err := s.seal(plain)
	if err != nil {
		return err
	}
	return s.SetMeta(metaTonnelEnc, enc)
}

func (s *Store) TonnelInitData() (string, error) {
	enc, err := s.GetMeta(metaTonnelEnc)
	if err != nil {
		return "", err
	}
	return s.open(enc)
}

func (s *Store) HasTonnelInitData() bool {
	v, _ := s.GetMeta(metaTonnelEnc)
	return v != ""
}

// Runtime — пороги и интервал из Bbolt (с дефолтами).
type Runtime struct {
	PollIntervalSec int   `json:"poll_interval_sec"`
	MinProfitNano   int64 `json:"min_profit_nano"`
	MinSpreadBPS    int   `json:"min_spread_bps"`
}

func DefaultRuntime() Runtime {
	return Runtime{
		PollIntervalSec: 1,
		MinProfitNano:   int64(money.DefaultMinProfit),
		MinSpreadBPS:    int(money.MinSpreadBPS),
	}
}

func (s *Store) GetRuntime() (Runtime, error) {
	r := DefaultRuntime()
	if v, err := s.GetMeta(metaPollSec); err == nil && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			r.PollIntervalSec = n
		}
	}
	if v, err := s.GetMeta(metaMinProfit); err == nil && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			r.MinProfitNano = n
		}
	}
	if v, err := s.GetMeta(metaMinSpreadBPS); err == nil && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			r.MinSpreadBPS = n
		}
	}
	return r, nil
}

func (s *Store) PutRuntime(r Runtime) error {
	if r.PollIntervalSec < 1 {
		return fmt.Errorf("store: poll_interval_sec min 1")
	}
	if r.MinSpreadBPS < 0 || r.MinProfitNano < 0 {
		return fmt.Errorf("store: negative thresholds")
	}
	if err := s.SetMeta(metaPollSec, strconv.Itoa(r.PollIntervalSec)); err != nil {
		return err
	}
	if err := s.SetMeta(metaMinProfit, strconv.FormatInt(r.MinProfitNano, 10)); err != nil {
		return err
	}
	return s.SetMeta(metaMinSpreadBPS, strconv.Itoa(r.MinSpreadBPS))
}

func (s *Store) PollInterval() time.Duration {
	r, _ := s.GetRuntime()
	return time.Duration(r.PollIntervalSec) * time.Second
}

// PutSlotJSON helper for API round-trip validation.
func SlotFromJSON(raw []byte) (catalog.WatchSlot, error) {
	var slot catalog.WatchSlot
	if err := json.Unmarshal(raw, &slot); err != nil {
		return slot, err
	}
	return slot, nil
}
