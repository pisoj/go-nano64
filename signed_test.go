package nano64

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSignedNano64_FromId_ToId(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
	}{
		{"zero", 0},
		{"sign bit boundary", 1 << 63},
		{"max", ^uint64(0)},
		{"random", 0x123456789ABCDEF0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := FromUint64(tt.value)

			signed := SignedNano64.FromId(original)
			roundtrip := SignedNano64.ToId(signed)

			if got := roundtrip.Uint64Value(); got != tt.value {
				t.Errorf(
					"FromId/ToId roundtrip failed: got %d, want %d",
					got,
					tt.value,
				)
			}
		})
	}
}

func TestSignedNano64_OrderPreservation(t *testing.T) {
	ids := []Nano64{
		New(0),
		New(1),
		New(1 << RandomBits),
		New(1 << (RandomBits + 10)),
		New(^uint64(0)),
	}

	for i := 1; i < len(ids); i++ {
		prev := SignedNano64.FromId(ids[i-1])
		curr := SignedNano64.FromId(ids[i])

		if prev >= curr {
			t.Errorf(
				"order not preserved: %d >= %d",
				prev,
				curr,
			)
		}
	}
}

func TestSignedNano64_GetTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		random    uint64
	}{
		{"zero", 0, 0},
		{"small timestamp", 123, 1},
		{"large timestamp", 1234567890123, 0xABCDE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := New((uint64(tt.timestamp) << RandomBits) | tt.random)
			signed := SignedNano64.FromId(id)

			if got := SignedNano64.GetTimestamp(signed); got != tt.timestamp {
				t.Errorf(
					"GetTimestamp() = %d, want %d",
					got,
					tt.timestamp,
				)
			}
		})
	}
}

func TestSignedNano64_TimeRange(t *testing.T) {
	tests := []struct {
		name       string
		startTs    int64
		endTs      int64
		expectSpan int
	}{
		{"single millisecond", 1000, 1000, 1},
		{"two milliseconds", 1000, 1001, 2},
		{"wide range", 500, 505, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := SignedNano64.TimeRange(tt.startTs, tt.endTs)
			if err != nil {
				t.Fatalf("TimeRange() error = %v", err)
			}

			if start >= end {
				t.Errorf("invalid range: start %d >= end %d", start, end)
			}
		})
	}
}

func TestSignedNano64_TimeRange_Errors(t *testing.T) {
	maxTimestamp := int64((1 << TimestampBits) - 1)

	tests := []struct {
		name    string
		start   int64
		end     int64
		wantErr bool
	}{
		{"negative start", -1, 100, true},
		{"negative end", 0, -1, true},
		{"start greater than end", 200, 100, true},
		{"start overflow", maxTimestamp + 1, maxTimestamp + 1, true},
		{"valid", 100, 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := SignedNano64.TimeRange(tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"TimeRange() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestSignedNano64_DatabaseRangeQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE events (
			id INTEGER PRIMARY KEY,
			ts INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	timestamps := []int64{1000, 2000, 3000}

	for _, ts := range timestamps {
		id := New(uint64(ts) << RandomBits)
		signed := SignedNano64.FromId(id)

		_, err := db.Exec(
			"INSERT INTO events (id, ts) VALUES (?, ?)",
			signed,
			ts,
		)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	start, end, err := SignedNano64.TimeRange(2000, 3000)
	if err != nil {
		t.Fatalf("TimeRange() error = %v", err)
	}

	rows, err := db.Query(
		"SELECT ts FROM events WHERE id BETWEEN ? AND ? ORDER BY id",
		start,
		end,
	)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var got []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		got = append(got, ts)
	}

	want := []int64{2000, 3000}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %d, want %d", i, got[i], want[i])
		}
	}
}
