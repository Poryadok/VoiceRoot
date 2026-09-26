package controlledgame

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const callbackPath = "/callbacks/game-commands/v1"

var errInvalidRequest = errors.New("invalid callback request")

// authenticateRequest checks the frozen callback envelope before any durable
// receiver work. body must already be the exact UTF-8 JCS representation.
func authenticateRequest(method, rawPath, timestamp, keyID string, body []byte, signature string, key []byte, now time.Time) error {
	if method != "POST" || !validCallbackPath(rawPath) || !canonicalUUID(keyID) || len(key) != 32 {
		return errInvalidRequest
	}
	timestampSeconds, err := canonicalUnixSeconds(timestamp)
	if err != nil || !withinSkew(timestampSeconds, now.UTC().Unix(), 300) {
		return errInvalidRequest
	}
	canonicalBody, err := canonicalJSON(body)
	if err != nil || !bytes.Equal(canonicalBody, body) {
		return errInvalidRequest
	}
	if len(signature) != len("v1=")+sha256.Size*2 || !strings.HasPrefix(signature, "v1=") {
		return errInvalidRequest
	}
	supplied, err := hex.DecodeString(signature[3:])
	if err != nil || hex.EncodeToString(supplied) != signature[3:] {
		return errInvalidRequest
	}
	bodyHash := sha256.Sum256(body)
	input := strings.Join([]string{"v1", method, rawPath, timestamp, keyID, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(input))
	if !hmac.Equal(supplied, mac.Sum(nil)) {
		return errInvalidRequest
	}
	return nil
}

func validCallbackPath(rawPath string) bool {
	if rawPath != callbackPath || strings.ContainsAny(rawPath, "?#%\\") || !utf8.ValidString(rawPath) {
		return false
	}
	parsed, err := url.ParseRequestURI(rawPath)
	return err == nil && parsed.Path == callbackPath && parsed.RawQuery == "" && parsed.Fragment == ""
}

func canonicalUnixSeconds(value string) (int64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, errInvalidRequest
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, errInvalidRequest
		}
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != value {
		return 0, errInvalidRequest
	}
	return seconds, nil
}

func withinSkew(timestamp, now int64, maxSkew uint64) bool {
	if timestamp < 0 || now < 0 {
		return false
	}
	if timestamp >= now {
		return uint64(timestamp-now) <= maxSkew
	}
	return uint64(now-timestamp) <= maxSkew
}

func canonicalUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func canonicalJSON(raw []byte) ([]byte, error) {
	if len(raw) == 0 || !utf8.Valid(raw) {
		return nil, errInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, errInvalidRequest
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errInvalidRequest
	}
	var output bytes.Buffer
	if err := appendCanonicalJSON(&output, value); err != nil {
		return nil, errInvalidRequest
	}
	return output.Bytes(), nil
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errInvalidRequest
				}
				if _, exists := object[key]; exists {
					return nil, errInvalidRequest
				}
				item, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				object[key] = item
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
				return nil, errInvalidRequest
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				item, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				array = append(array, item)
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
				return nil, errInvalidRequest
			}
			return array, nil
		default:
			return nil, errInvalidRequest
		}
	case json.Number, string, bool, nil:
		return value, nil
	default:
		return nil, errInvalidRequest
	}
}

func appendCanonicalJSON(output *bytes.Buffer, value any) error {
	switch item := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendJSONString(output, key); err != nil {
				return err
			}
			output.WriteByte(':')
			if err := appendCanonicalJSON(output, item[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	case []any:
		output.WriteByte('[')
		for index, value := range item {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendCanonicalJSON(output, value); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case string:
		return appendJSONString(output, item)
	case json.Number:
		return appendCanonicalInteger(output, item.String())
	case bool:
		if item {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case nil:
		output.WriteString("null")
	default:
		return errInvalidRequest
	}
	return nil
}

func appendCanonicalInteger(output *bytes.Buffer, number string) error {
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || math.Trunc(value) != value || math.Abs(value) > 9007199254740991 {
		return errInvalidRequest
	}
	if value == 0 {
		output.WriteByte('0')
		return nil
	}
	output.WriteString(strconv.FormatFloat(value, 'f', 0, 64))
	return nil
}

func appendJSONString(output *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return errInvalidRequest
	}
	output.WriteByte('"')
	for _, char := range value {
		switch char {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(char)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if char < 0x20 {
				output.WriteString(`\u00`)
				output.WriteByte("0123456789abcdef"[byte(char)>>4])
				output.WriteByte("0123456789abcdef"[byte(char)&0x0f])
			} else {
				output.WriteRune(char)
			}
		}
	}
	output.WriteByte('"')
	return nil
}

func utf16Less(left, right string) bool {
	leftUnits := utf16.Encode([]rune(left))
	rightUnits := utf16.Encode([]rune(right))
	limit := len(leftUnits)
	if len(rightUnits) < limit {
		limit = len(rightUnits)
	}
	for index := 0; index < limit; index++ {
		if leftUnits[index] != rightUnits[index] {
			return leftUnits[index] < rightUnits[index]
		}
	}
	return len(leftUnits) < len(rightUnits)
}
