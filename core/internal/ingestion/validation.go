package ingestion

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Keep the embedded runtime schema identical to the shared contract.
//
//go:generate cp ../../../docs/contracts/ingestion/v1/schema.json schema.json
//go:embed schema.json
var schemaJSON []byte

func compileSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const location = "urn:inflacao-real:ingestion:v1"
	if err := compiler.AddResource(location, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

func validatePayload(schema *jsonschema.Schema, body []byte) (Submission, error) {
	if !json.Valid(body) || !utf8.Valid(body) || !validEscapes(body) {
		return Submission{}, errors.New("invalid JSON or Unicode")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	doc, err := readValue(decoder, 0)
	if err != nil {
		return Submission{}, err
	}
	if err := schema.Validate(doc); err != nil {
		return Submission{}, err
	}
	envelope := doc.(map[string]any)
	seen := make(map[string]bool)
	for _, value := range envelope["observations"].([]any) {
		observation := value.(map[string]any)
		id := observation["observation_id"].(string)
		if seen[id] {
			return Submission{}, errors.New("duplicate observation ID")
		}
		seen[id] = true
		for _, field := range []string{"source_url", "product_url"} {
			u, err := url.Parse(observation[field].(string))
			if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return Submission{}, fmt.Errorf("invalid %s", field)
			}
		}
	}
	return Submission{
		ScraperID:     envelope["scraper_id"].(string),
		SubmissionID:  envelope["submission_id"].(string),
		ShopID:        envelope["shop_id"].(string),
		SchemaVersion: envelope["schema_version"].(string),
		CreatedAt:     envelope["created_at"].(string),
		Payload:       body,
	}, nil
}

// encoding/json replaces invalid surrogate escapes. Reject them before decoding,
// along with U+0000, which PostgreSQL JSONB cannot represent.
func validEscapes(body []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}
		i++
		if body[i] != 'u' {
			continue
		}
		code, _ := strconv.ParseUint(string(body[i+1:i+5]), 16, 16)
		i += 4
		if code == 0 || (code >= 0xdc00 && code <= 0xdfff) {
			return false
		}
		if code >= 0xd800 && code <= 0xdbff {
			if i+6 >= len(body) || body[i+1] != '\\' || body[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(body[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

// Decode without silently accepting duplicate object keys. The depth cap also
// bounds work on malformed envelopes before schema validation.
func readValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("JSON nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			name := key.(string)
			if _, exists := object[name]; exists {
				return nil, errors.New("duplicate JSON object key")
			}
			value, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		_, err = decoder.Token()
		return object, err
	case json.Delim('['):
		array := make([]any, 0)
		for decoder.More() {
			value, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		_, err = decoder.Token()
		return array, err
	default:
		return token, nil
	}
}
