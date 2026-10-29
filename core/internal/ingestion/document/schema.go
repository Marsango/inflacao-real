package document

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:generate cp ../../../../docs/contracts/ingestion/v1/schema.json schema.json
//go:embed schema.json
var SchemaJSON []byte

func CompileSchema() (*jsonschema.Schema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const location = "urn:inflacao-real:ingestion:v1"
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

func ValidatePayload(schema *jsonschema.Schema, body []byte) (Submission, error) {
	document, err := decodeDocument(body)
	if err != nil {
		return Submission{}, err
	}
	if err := schema.Validate(document); err != nil {
		return Submission{}, err
	}
	envelope := document.(map[string]any)
	if err := validateObservations(envelope["observations"].([]any)); err != nil {
		return Submission{}, err
	}
	return submissionMetadata(envelope, body), nil
}

func decodeDocument(body []byte) (any, error) {
	if !json.Valid(body) || !utf8.Valid(body) || !validEscapes(body) {
		return nil, errors.New("invalid JSON or Unicode")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	reader := documentReader{decoder: decoder}
	value := reader.readValue(0)
	return value, reader.err
}

func submissionMetadata(envelope map[string]any, body []byte) Submission {
	return Submission{
		ScraperID: envelope["scraper_id"].(string), SubmissionID: envelope["submission_id"].(string),
		ShopID: envelope["shop_id"].(string), SchemaVersion: envelope["schema_version"].(string),
		CreatedAt: envelope["created_at"].(string), Payload: body,
	}
}

func validateObservations(observations []any) error {
	seen := make(map[string]bool)
	var err error
	for position := 0; position < len(observations) && err == nil; position++ {
		err = validateObservation(observations[position].(map[string]any), seen)
	}
	return err
}

func validateObservation(observation map[string]any, seen map[string]bool) error {
	identity := observation["observation_id"].(string)
	if seen[identity] {
		return errors.New("duplicate observation ID")
	}
	seen[identity] = true
	if err := validateURL(observation["source_url"].(string), "source_url"); err != nil {
		return err
	}
	return validateURL(observation["product_url"].(string), "product_url")
}

func validateURL(value, field string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid %s", field)
	}
	if parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("invalid %s", field)
	}
	return nil
}
