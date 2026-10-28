package grpcsvc

import (
	"bytes"
	"encoding/binary"
)

// Inspect container chunks before the image decoder, which may otherwise decode
// only an animation's first frame. The processor still validates image payloads.
func staticStickerSource(raw []byte, mime string) bool {
	if mime == "image/png" {
		if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
			return false
		}
		for offset := 8; offset+12 <= len(raw); {
			n := uint64(binary.BigEndian.Uint32(raw[offset:]))
			if n > uint64(len(raw)-offset-12) {
				return false
			}
			kind := string(raw[offset+4 : offset+8])
			if kind == "acTL" {
				return false
			}
			offset += int(n) + 12
			if kind == "IEND" {
				return offset == len(raw)
			}
		}
		return false
	}
	if mime != "image/webp" || len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(raw[4:8]))+8 != uint64(len(raw)) {
		return false
	}
	for offset := 12; offset+8 <= len(raw); {
		n := uint64(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		if n > uint64(len(raw)-offset-8) {
			return false
		}
		kind := string(raw[offset : offset+4])
		if kind == "ANIM" || kind == "ANMF" || (kind == "VP8X" && n > 0 && raw[offset+8]&2 != 0) {
			return false
		}
		offset += 8 + int(n) + int(n%2)
		if offset == len(raw) {
			return true
		}
	}
	return false
}
