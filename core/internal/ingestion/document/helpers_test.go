package document

import (
	"bytes"
	"os"
	"testing"
)

func example(test *testing.T, name string) []byte {
	test.Helper()
	body, err := os.ReadFile("../../../../docs/contracts/ingestion/v1/examples/" + name + ".json")
	if err != nil {
		test.Fatal(err)
	}
	return body
}

func TestEmbeddedSchemaMatchesContract(test *testing.T) {
	expected, err := os.ReadFile("../../../../docs/contracts/ingestion/v1/schema.json")
	if err != nil {
		test.Fatal(err)
	}
	if !bytes.Equal(SchemaJSON, expected) {
		test.Fatal("runtime schema differs from contract; run go generate ./internal/ingestion/document")
	}
}
