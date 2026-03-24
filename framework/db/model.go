package db

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"
)

// BaseModel provides the standard fields every domain model includes.
// Embed it as the first field in every model struct. The scanner reads
// these columns from the database automatically via db tags.
type BaseModel struct {
	ID        string     `db:"id"         json:"id"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ---------------------------------------------------------------------------
// ID generation — zero-dependency, time-sortable, 20-char alphanumeric
// ---------------------------------------------------------------------------

// NewID generates a unique, time-sortable identifier suitable for use as
// a primary key. Format: 8 chars timestamp (base32) + 12 chars random.
// Total: 20 characters, alphanumeric-safe, lexicographically sortable by
// creation time.
func NewID() string {
	return generateID(time.Now())
}

// alphabet is a 32-char set for base32 encoding (Crockford-inspired).
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

var idCounter atomic.Uint64

func generateID(now time.Time) string {
	var buf [20]byte

	// Timestamp component: milliseconds since epoch, encoded in base32.
	// 8 chars of base32 = 40 bits = ~34 years of millisecond precision
	// from a recent epoch, which is plenty for Sunkern's use case.
	ms := uint64(now.UnixMilli())
	for i := 7; i >= 0; i-- {
		buf[i] = alphabet[ms&0x1f]
		ms >>= 5
	}

	// Random component: 12 chars of random base32.
	var randomBytes [8]byte
	_, _ = rand.Read(randomBytes[:])

	// Mix in a counter to guarantee uniqueness even with same timestamp
	// and same random bytes (extremely unlikely but defensive).
	counter := idCounter.Add(1)
	binary.LittleEndian.PutUint64(randomBytes[:], binary.LittleEndian.Uint64(randomBytes[:])+counter)

	for i := 0; i < 12; i++ {
		// Use 5 bits from the random pool per character.
		byteIdx := (i * 5) / 8
		bitOffset := uint((i * 5) % 8)

		var val uint16
		if byteIdx+1 < len(randomBytes) {
			val = uint16(randomBytes[byteIdx]) | uint16(randomBytes[byteIdx+1])<<8
		} else {
			val = uint16(randomBytes[byteIdx])
		}
		buf[8+i] = alphabet[(val>>bitOffset)&0x1f]
	}

	return string(buf[:])
}

// FormatTime formats a time.Time in the canonical SQLite TEXT format.
func FormatTime(t time.Time) string {
	return t.Format(timeFormat)
}

// NowFormatted returns the current time formatted for SQLite TEXT columns.
func NowFormatted() string {
	return FormatTime(time.Now())
}

// Stringer for debugging.
func (m BaseModel) String() string {
	return fmt.Sprintf("BaseModel{ID: %s}", m.ID)
}
