// Package record implements SQLite's varint and record encodings. All numbers
// on disk are big-endian; SQLite varints are not encoding/binary's varints.
package record

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var (
	ErrCorrupt     = errors.New("corrupt SQLite record")
	ErrUnsupported = errors.New("unsupported SQLite record value")
	ErrLimit       = errors.New("SQLite record resource limit")
)

// These limits bound allocations made from untrusted on-disk lengths.
const MaxPayload = 64 << 20
const MaxColumns = 32767

// DecodeVarint consumes at most nine bytes. The ninth carries eight data bits.
func DecodeVarint(p []byte) (uint64, int, error) {
	var v uint64
	for i := 0; i < 8; i++ {
		if i >= len(p) {
			return 0, 0, fmt.Errorf("%w: truncated varint", ErrCorrupt)
		}
		v = v<<7 | uint64(p[i]&127)
		if p[i]&128 == 0 {
			return v, i + 1, nil
		}
	}
	if len(p) < 9 {
		return 0, 0, fmt.Errorf("%w: truncated nine-byte varint", ErrCorrupt)
	}
	return v<<8 | uint64(p[8]), 9, nil
}

func varintLen(v uint64) int {
	if v > 0x00ffffffffffffff {
		return 9
	}
	n := 1
	for v >= 128 {
		n++
		v >>= 7
	}
	return n
}

// AppendVarint appends the shortest SQLite representation of v.
func AppendVarint(dst []byte, v uint64) []byte {
	var buf [9]byte
	n := varintLen(v)
	if n == 9 {
		buf[8] = byte(v)
		v >>= 8
		for i := 7; i >= 0; i-- {
			buf[i] = byte(v&127) | 128
			v >>= 7
		}
	} else {
		for i := n - 1; i >= 0; i-- {
			buf[i] = byte(v & 127)
			if i < n-1 {
				buf[i] |= 128
			}
			v >>= 7
		}
	}
	return append(dst, buf[:n]...)
}

func serialLength(t uint64) (uint64, error) {
	switch t {
	case 0, 8, 9:
		return 0, nil
	case 1, 2, 3, 4:
		return t, nil
	case 5:
		return 6, nil
	case 6, 7:
		return 8, nil
	case 10, 11:
		return 0, fmt.Errorf("%w: reserved serial type %d", ErrCorrupt, t)
	default:
		return (t - 12) / 2, nil
	}
}

// Decode returns native SQLite storage classes: nil, int64, float64, string,
// and []byte. BLOBs and text own their storage, including non-nil empty BLOBs.
// Strings preserve bytes; this layer does not normalize or repair UTF-8.
func Decode(p []byte) ([]any, error) {
	if len(p) > MaxPayload {
		return nil, ErrLimit
	}
	header, n, err := DecodeVarint(p)
	if err != nil {
		return nil, err
	}
	if header < uint64(n) || header > uint64(len(p)) {
		return nil, fmt.Errorf("%w: invalid header length", ErrCorrupt)
	}
	pos, end, data := n, int(header), int(header)
	values := make([]any, 0, 8)
	for pos < end {
		if len(values) >= MaxColumns {
			return nil, ErrLimit
		}
		t, k, err := DecodeVarint(p[pos:end])
		if err != nil {
			return nil, err
		}
		pos += k
		size, err := serialLength(t)
		if err != nil {
			return nil, err
		}
		if size > uint64(len(p)-data) {
			return nil, fmt.Errorf("%w: truncated field %d", ErrCorrupt, len(values))
		}
		field := p[data : data+int(size)]
		data += int(size)
		var v any
		switch {
		case t == 0:
		case t >= 1 && t <= 6:
			var u uint64
			for _, b := range field {
				u = u<<8 | uint64(b)
			}
			if len(field) < 8 && field[0]&128 != 0 {
				u |= ^uint64(0) << uint(len(field)*8)
			}
			v = int64(u)
		case t == 7:
			f := math.Float64frombits(binary.BigEndian.Uint64(field))
			// SQLite converts IEEE NaN serial values into SQL NULL.
			if !math.IsNaN(f) {
				v = f
			}
		case t == 8:
			v = int64(0)
		case t == 9:
			v = int64(1)
		case t&1 == 0:
			b := make([]byte, len(field))
			copy(b, field)
			v = b
		default:
			v = string(field)
		}
		values = append(values, v)
	}
	if data != len(p) {
		return nil, fmt.Errorf("%w: trailing payload bytes", ErrCorrupt)
	}
	return values, nil
}

func serialFor(v any) (uint64, int, error) {
	switch x := v.(type) {
	case nil:
		return 0, 0, nil
	case int64:
		if x == 0 || x == 1 {
			return uint64(8 + x), 0, nil
		}
		switch {
		case x >= -128 && x <= 127:
			return 1, 1, nil
		case x >= -32768 && x <= 32767:
			return 2, 2, nil
		case x >= -8388608 && x <= 8388607:
			return 3, 3, nil
		case x >= -2147483648 && x <= 2147483647:
			return 4, 4, nil
		case x >= -140737488355328 && x <= 140737488355327:
			return 5, 6, nil
		default:
			return 6, 8, nil
		}
	case float64:
		if math.IsNaN(x) {
			return 0, 0, nil
		}
		return 7, 8, nil
	case string:
		return 13 + uint64(len(x))*2, len(x), nil
	case []byte:
		return 12 + uint64(len(x))*2, len(x), nil
	default:
		return 0, 0, fmt.Errorf("%w: %T", ErrUnsupported, v)
	}
}

// Encode produces canonical serial types and the minimal record header.
// Integer inputs must be int64; NaN is stored as NULL, matching SQLite.
func Encode(values []any) ([]byte, error) {
	if len(values) > MaxColumns {
		return nil, ErrLimit
	}
	types := make([]uint64, len(values))
	sizes := make([]int, len(values))
	headerBytes, bodyBytes := 0, 0
	for i, v := range values {
		t, n, e := serialFor(v)
		if e != nil {
			return nil, e
		}
		if n > MaxPayload-bodyBytes {
			return nil, ErrLimit
		}
		types[i], sizes[i] = t, n
		headerBytes += varintLen(t)
		bodyBytes += n
	}
	header := headerBytes + 1
	for varintLen(uint64(header))+headerBytes != header {
		header = varintLen(uint64(header)) + headerBytes
	}
	if header > MaxPayload-bodyBytes {
		return nil, ErrLimit
	}
	out := make([]byte, 0, header+bodyBytes)
	out = AppendVarint(out, uint64(header))
	for _, t := range types {
		out = AppendVarint(out, t)
	}
	var buf [8]byte
	for i, v := range values {
		n := sizes[i]
		if n == 0 {
			continue
		}
		switch x := v.(type) {
		case int64:
			binary.BigEndian.PutUint64(buf[:], uint64(x))
			out = append(out, buf[8-n:]...)
		case float64:
			binary.BigEndian.PutUint64(buf[:], math.Float64bits(x))
			out = append(out, buf[:]...)
		case string:
			out = append(out, x...)
		case []byte:
			out = append(out, x...)
		}
	}
	return out, nil
}
