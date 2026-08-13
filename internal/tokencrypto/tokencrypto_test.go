package tokencrypto_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/reqiewu/order-bot/internal/tokencrypto"
)

func TestSealOpenRoundTrip(t *testing.T) {
	t.Parallel()
	key, err := tokencrypto.ParseKey(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := tokencrypto.Seal(key, []byte("secret-token"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("secret-token")) {
		t.Fatal("plaintext leaked into ciphertext blob")
	}
	plain, err := tokencrypto.Open(key, blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "secret-token" {
		t.Fatalf("got %q", plain)
	}
}

func TestParseKeySHA256Fallback(t *testing.T) {
	t.Parallel()
	a, err := tokencrypto.ParseKey("change-me")
	if err != nil || len(a) != 32 {
		t.Fatalf("a=%v err=%v", a, err)
	}
	b, err := tokencrypto.ParseKey("change-me")
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("same input must yield same key")
	}
}

func TestOpenRejectsTamper(t *testing.T) {
	t.Parallel()
	key, _ := tokencrypto.ParseKey("k")
	blob, err := tokencrypto.Seal(key, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	blob[len(blob)-1] ^= 0xff
	if _, err := tokencrypto.Open(key, blob); err == nil {
		t.Fatal("expected open error")
	}
}
