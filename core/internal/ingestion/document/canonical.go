package document

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
)

// Canonicalization covers schema-validated v1 data: fixed ASCII keys and bounded
// integer JSON numbers. Money stays in strings; this is not a general JCS encoder.
func WriteCanonical(output *bytes.Buffer, value any) {
	switch value := value.(type) {
	case map[string]any:
		writeObject(output, value)
	case []any:
		writeArray(output, value)
	case string:
		writeString(output, value)
	default:
		writePrimitive(output, value)
	}
}

func writeObject(output *bytes.Buffer, value map[string]any) {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output.WriteByte('{')
	for position, key := range keys {
		writeMember(output, value, key, position)
	}
	output.WriteByte('}')
}

func writeMember(output *bytes.Buffer, value map[string]any, key string, position int) {
	writeSeparator(output, position)
	WriteCanonical(output, key)
	output.WriteByte(':')
	WriteCanonical(output, value[key])
}

func writeArray(output *bytes.Buffer, value []any) {
	output.WriteByte('[')
	for position, item := range value {
		writeItem(output, item, position)
	}
	output.WriteByte(']')
}

func writeItem(output *bytes.Buffer, value any, position int) {
	writeSeparator(output, position)
	WriteCanonical(output, value)
}

func writeSeparator(output *bytes.Buffer, position int) {
	if position > 0 {
		output.WriteByte(',')
	}
}

var shortEscapes = map[rune]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\t': `\t`,
	'\n': `\n`, '\f': `\f`, '\r': `\r`}

func writeString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, character := range value {
		writeCharacter(output, character)
	}
	output.WriteByte('"')
}

func writeCharacter(output *bytes.Buffer, character rune) {
	if escaped, exists := shortEscapes[character]; exists {
		output.WriteString(escaped)
		return
	}
	if character < 0x20 {
		fmt.Fprintf(output, `\u%04x`, character)
		return
	}
	output.WriteRune(character)
}

func writePrimitive(output *bytes.Buffer, value any) {
	switch value := value.(type) {
	case float64:
		output.WriteString(strconv.FormatFloat(value, 'f', 0, 64))
	case bool:
		output.WriteString(strconv.FormatBool(value))
	case nil:
		output.WriteString("null")
	default:
		panic("unexpected value in validated v1 JSON")
	}
}

func Fingerprint(shopID string, canonicalObservation []byte) []byte {
	var output bytes.Buffer
	output.WriteString(`{"observation":`)
	output.Write(canonicalObservation)
	output.WriteString(`,"shop_id":`)
	WriteCanonical(&output, shopID)
	output.WriteByte('}')
	sum := sha256.Sum256(output.Bytes())
	return sum[:]
}
