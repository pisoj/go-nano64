package nano64

import "fmt"

// SignedNano64 is a utility for converting `Nano64` IDs to and from `int64`.
// This is particularly useful when storing Nano64 IDs in database columns that use
// a signed 64-bit integer type, such as PostgreSQL's `BIGINT` and SQLite's `INTEGER`.
// The conversion method used (`value XOR 2^63`) ensures that the natural sort order
// of the IDs is preserved, allowing for efficient, indexed range queries.
var SignedNano64 = signedNano64{}

type signedNano64 struct{}

// signBit is strictly uint64 because calculations always need to be performed
// in the unsigned domain to avoid triggering overflow compiler warnings.
const signBit uint64 = 1 << 63

// FromId returns signed int representation of the `id`,
// while perfectly preserving time-sorting properties.
func (signedNano64) FromId(id Nano64) int64 {
	return int64(id.Uint64Value() ^ signBit)
}

// ToId returns the u64 representation of the `signedIntId`.
func (signedNano64) ToId(signedIntId int64) Nano64 {
	unsignedInt64 := uint64(signedIntId) ^ signBit
	return FromUint64(unsignedInt64)
}

// TimeRange returns `start` and `end` signed int values for a database query based on a timestamp range.
// The returned values can be used directly in a SQL `BETWEEN` clause on a signed integer column.
func (signedNano64) TimeRange(timestampStart int64, timestampEnd int64) (int64, int64, error) {
	if timestampStart < 0 || timestampEnd < 0 {
		return 0, 0, fmt.Errorf("timestamps must be non-negative: start %d, end %d", timestampStart, timestampEnd)
	}
	if timestampStart > timestampEnd {
		return 0, 0, fmt.Errorf("timestampStart must be less than or equal to timestampEnd")
	}

	timestampMax := int64(timestampMask)
	if timestampStart > timestampMax || timestampEnd > timestampMax {
		return 0, 0, fmt.Errorf("timestamp exceeds the %d-bit range", TimestampBits)
	}

	randomMax := (uint64(1) << RandomBits) ^ 1
	unsignedStart := uint64(timestampStart) << RandomBits
	unsignedEnd := (uint64(timestampEnd) << RandomBits) | randomMax

	// Convert the unsigned bounds to signed bounds
	return int64(unsignedStart ^ signBit), int64(unsignedEnd ^ signBit), nil
}

// GetTimestamp extracts the embedded UNIX-epoch milliseconds from an ID represented as a signed integer.
// Returns integer milliseconds in range [0, 2^44-1].
func (signedNano64) GetTimestamp(signedIntId int64) int64 {
	// Convert the signed value back to its original unsigned representation.
	unsignedValue := uint64(signedIntId) ^ signBit

	// Right-shift the unsigned value to discard the 20 random bits,
	// moving the timestamp into the least significant position.
	return int64(unsignedValue >> RandomBits)
}
