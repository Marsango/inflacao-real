package document

import (
	"encoding/json"
	"errors"
	"strconv"
)

type documentReader struct {
	decoder *json.Decoder
	err     error
}

func (reader *documentReader) token() json.Token {
	token, err := reader.decoder.Token()
	reader.err = err
	return token
}

func (reader *documentReader) more() bool {
	return reader.err == nil && reader.decoder.More()
}

func (reader *documentReader) finish() {
	if reader.err == nil {
		reader.token()
	}
}

func (reader *documentReader) readValue(depth int) any {
	if depth > 64 {
		reader.err = errors.New("JSON nesting exceeds limit")
		return nil
	}
	token := reader.token()
	switch token {
	case json.Delim('{'):
		return reader.readObject(depth)
	case json.Delim('['):
		return reader.readArray(depth)
	default:
		return token
	}
}

func (reader *documentReader) readObject(depth int) map[string]any {
	object := make(map[string]any)
	for reader.more() {
		reader.readMember(object, depth)
	}
	reader.finish()
	return object
}

func (reader *documentReader) readMember(object map[string]any, depth int) {
	token := reader.token()
	if reader.err != nil {
		return
	}
	name := token.(string)
	if _, exists := object[name]; exists {
		reader.err = errors.New("duplicate JSON object key")
		return
	}
	object[name] = reader.readValue(depth + 1)
}

func (reader *documentReader) readArray(depth int) []any {
	array := make([]any, 0)
	for reader.more() {
		array = append(array, reader.readValue(depth+1))
	}
	reader.finish()
	return array
}

type unicodeScanner struct {
	body     []byte
	position int
	valid    bool
}

// JSON syntax has already been checked. Reject NUL and invalid surrogate pairs.
func validEscapes(body []byte) bool {
	scanner := unicodeScanner{body: body, valid: true}
	for scanner.valid && scanner.position < len(body) {
		scanner.advance()
	}
	return scanner.valid
}

func (scanner *unicodeScanner) advance() {
	if scanner.body[scanner.position] != '\\' {
		scanner.position++
		return
	}
	scanner.position++
	if scanner.body[scanner.position] != 'u' {
		scanner.position++
		return
	}
	code, _ := strconv.ParseUint(string(scanner.body[scanner.position+1:scanner.position+5]), 16, 16)
	scanner.position += 5
	if code == 0 || (code >= 0xdc00 && code <= 0xdfff) {
		scanner.valid = false
		return
	}
	if code >= 0xd800 && code <= 0xdbff {
		scanner.readLowSurrogate()
	}
}

func (scanner *unicodeScanner) readLowSurrogate() {
	position := scanner.position
	if position+6 > len(scanner.body) || scanner.body[position] != '\\' || scanner.body[position+1] != 'u' {
		scanner.valid = false
		return
	}
	low, err := strconv.ParseUint(string(scanner.body[position+2:position+6]), 16, 16)
	scanner.valid = err == nil && low >= 0xdc00 && low <= 0xdfff
	scanner.position += 6
}
