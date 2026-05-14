package entity

import (
	"errors"
	"fmt"
)

// JSONText is a []byte that doubles as a sql.Scanner + json.Marshaler
// for opaque JSON payloads.
//
// The Go stdlib refuses to Scan a driver-returned `string` into a
// `*json.RawMessage`, which is what happens when a SQLite column was
// added via `ALTER TABLE … TEXT` (the go-sqlite3 driver returns TEXT
// columns as Go `string`). Columns declared `JSON` in the original
// CREATE TABLE come back as `[]byte` instead — hence the historical
// `Roles json.RawMessage` works for the old column but not for
// grantable_roles / resource_ids, which were added later.
//
// JSONText handles both driver-value shapes and still emits the
// stored bytes verbatim when the struct is marshalled (no base64
// wrapping you'd get from a plain `[]byte`).
type JSONText []byte

// Scan implements sql.Scanner.
func (j *JSONText) Scan(src interface{}) error {
	if src == nil {
		*j = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		// Copy — the driver is allowed to reuse the source buffer
		// after Scan returns, so we can't keep a reference.
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], v...)
	default:
		return fmt.Errorf("JSONText: unsupported driver value type %T", src)
	}
	return nil
}

// MarshalJSON emits the stored bytes verbatim so API responses carry
// the underlying array/object, not a base64-encoded string.
func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON stores the input bytes as-is — matches json.RawMessage.
func (j *JSONText) UnmarshalJSON(b []byte) error {
	if j == nil {
		return errors.New("JSONText: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[:0], b...)
	return nil
}
