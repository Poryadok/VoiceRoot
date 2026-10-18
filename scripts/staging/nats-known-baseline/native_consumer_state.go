package main

// Format contract: nats-server v2.12.12 server/store.go encodeConsumerState and
// server/filestore.go decodeConsumerState. This decodes CLOSED durable state,
// not live consumer callback/queue state and not authenticated writer identity.
import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
)

type nativeSequence struct {
	Consumer uint64 `json:"consumer"`
	Stream   uint64 `json:"stream"`
}
type nativePending struct {
	ConsumerSequence uint64 `json:"consumer_sequence"`
	TimestampNS      int64  `json:"timestamp_ns"`
}
type nativeDurableState struct {
	Schema      string                   `json:"schema"`
	Version     byte                     `json:"version"`
	AckFloor    nativeSequence           `json:"ack_floor"`
	Delivered   nativeSequence           `json:"delivered"`
	Pending     map[uint64]nativePending `json:"pending"`
	Redelivered map[uint64]uint64        `json:"redelivered"`
}

var errNativeState = errors.New("native_consumer_state_refused")

func decodeNativeConsumerState(raw []byte) (nativeDurableState, error) {
	s := nativeDurableState{Schema: "nats-closed-durable-consumer-v1", Pending: map[uint64]nativePending{}, Redelivered: map[uint64]uint64{}}
	if len(raw) < 2 || len(raw) > 256<<10 || raw[0] != 22 || (raw[1] != 1 && raw[1] != 2) {
		return s, errNativeState
	}
	s.Version = raw[1]
	off := 2
	u := func() (uint64, error) {
		n, k := binary.Uvarint(raw[off:])
		if k <= 0 || n > math.MaxInt64 {
			return 0, errNativeState
		}
		var buf [10]byte
		size := binary.PutUvarint(buf[:], n)
		if size != k || !bytes.Equal(raw[off:off+k], buf[:size]) {
			return 0, errNativeState
		}
		off += k
		return n, nil
	}
	i := func() (int64, error) {
		n, k := binary.Varint(raw[off:])
		if k <= 0 {
			return 0, errNativeState
		}
		var buf [10]byte
		size := binary.PutVarint(buf[:], n)
		if size != k || !bytes.Equal(raw[off:off+k], buf[:size]) {
			return 0, errNativeState
		}
		off += k
		return n, nil
	}
	add := func(a, b uint64) (uint64, error) {
		if a > math.MaxInt64-b {
			return 0, errNativeState
		}
		return a + b, nil
	}
	for _, v := range []*uint64{&s.AckFloor.Consumer, &s.AckFloor.Stream, &s.Delivered.Consumer, &s.Delivered.Stream} {
		n, e := u()
		if e != nil {
			return s, e
		}
		*v = n
	}
	if s.Version == 1 {
		for _, pair := range [][2]*uint64{{&s.Delivered.Consumer, &s.AckFloor.Consumer}, {&s.Delivered.Stream, &s.AckFloor.Stream}} {
			base := *pair[1]
			if base > 0 {
				base--
			}
			n, e := add(*pair[0], base)
			if e != nil {
				return s, e
			}
			*pair[0] = n
		}
	}
	if s.AckFloor.Consumer > s.Delivered.Consumer || s.AckFloor.Stream > s.Delivered.Stream {
		return s, errNativeState
	}
	count, e := u()
	if e != nil || count > 16384 {
		return s, errNativeState
	}
	if count > 0 {
		base, e := i()
		if e != nil {
			return s, e
		}
		for n := uint64(0); n < count; n++ {
			delta, e := u()
			if e != nil || delta == 0 {
				return s, errNativeState
			}
			seq, e := add(delta, s.AckFloor.Stream)
			if e != nil || seq > s.Delivered.Stream {
				return s, errNativeState
			}
			var cseq uint64
			if s.Version == 2 {
				cd, e := u()
				if e != nil || cd == 0 {
					return s, errNativeState
				}
				cseq, e = add(cd, s.AckFloor.Consumer)
				if e != nil || cseq > s.Delivered.Consumer {
					return s, errNativeState
				}
			}
			dt, e := i()
			if e != nil {
				return s, e
			}
			// Use checked signed addition/subtraction before nanosecond expansion.
			if s.Version == 2 {
				if dt == math.MinInt64 {
					return s, errNativeState
				}
				dt = -dt
			}
			if (dt > 0 && base > math.MaxInt64-dt) || (dt < 0 && base < math.MinInt64-dt) {
				return s, errNativeState
			}
			sec := base + dt
			if sec < 0 || sec > math.MaxInt64/1000000000 {
				return s, errNativeState
			}
			if _, exists := s.Pending[seq]; exists {
				return s, errNativeState
			}
			s.Pending[seq] = nativePending{cseq, sec * 1000000000}
		}
	}
	count, e = u()
	if e != nil || count > 16384 {
		return s, errNativeState
	}
	for n := uint64(0); n < count; n++ {
		delta, e := u()
		if e != nil || delta == 0 {
			return s, errNativeState
		}
		seq, e := add(delta, s.AckFloor.Stream)
		if e != nil || seq > s.Delivered.Stream {
			return s, errNativeState
		}
		times, e := u()
		if e != nil || times == 0 {
			return s, errNativeState
		}
		if _, exists := s.Redelivered[seq]; exists {
			return s, errNativeState
		}
		s.Redelivered[seq] = times
	}
	if off != len(raw) {
		return s, errNativeState
	}
	return s, nil
}

func nativeConsumerState(args []string, r io.Reader, w io.Writer) error {
	if len(args) != 0 {
		return errNativeState
	}
	raw, e := io.ReadAll(io.LimitReader(r, (256<<10)+1))
	if e != nil {
		return errNativeState
	}
	s, e := decodeNativeConsumerState(raw)
	if e != nil {
		return e
	}
	return json.NewEncoder(w).Encode(s)
}
