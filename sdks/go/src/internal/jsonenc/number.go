package jsonenc

import "strconv"

// AppendInt64 appends the decimal representation of v to dst.
func AppendInt64(dst []byte, v int64) []byte {
	return strconv.AppendInt(dst, v, 10)
}

// AppendUint64 appends the decimal representation of v to dst.
func AppendUint64(dst []byte, v uint64) []byte {
	return strconv.AppendUint(dst, v, 10)
}

// AppendFloat64 appends v to dst as a JSON number.
// NaN and Inf are encoded as null to stay valid JSON.
func AppendFloat64(dst []byte, v float64) []byte {
	switch {
	case v != v: // NaN
		return append(dst, "null"...)
	case v > 1e308 || v < -1e308: // Inf
		return append(dst, "null"...)
	}
	return strconv.AppendFloat(dst, v, 'f', -1, 64)
}
