package document

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestGTINCheckDigits(test *testing.T) {
	for _, scenario := range []struct {
		value string
		valid bool
	}{
		{"96385074", true}, {"036000291452", true}, {"4006381333931", true},
		{"10012345000017", true}, {"4006381333932", false}, {"96385075", false},
		{"036000291453", false}, {"10012345000018", false}, {"123", false},
		{"400638133393x", false}, {"", false},
	} {
		test.Run(scenario.value, func(test *testing.T) {
			if actual := ValidGTIN(scenario.value); actual != scenario.valid {
				test.Fatalf("ValidGTIN(%q)=%t, want %t", scenario.value, actual, scenario.valid)
			}
		})
	}
}

func requireNoError(test *testing.T, err error) {
	test.Helper()
	if err != nil {
		test.Fatal(err)
	}
}

func testSchema(test *testing.T) *jsonschema.Schema {
	schema, err := CompileSchema()
	requireNoError(test, err)
	return schema
}

type domainCase struct {
	name, amount, reference, gtin string
	valid                         bool
}

func domainCases() []domainCase {
	return []domainCase{
		{"maximum", "999999999.99", "999999999.99", "4006381333931", true},
		{"zero", "0.00", "0.00", "036000291452", true},
		{"one cent below", "999999999.99", "999999999.98", "4006381333931", false},
		{"smaller reference", "24.90", "9.99", "4006381333931", false},
		{"bad GTIN", "24.90", "27.90", "4006381333932", false},
	}
}

func TestWorkerDomainValidationAndExactPrices(test *testing.T) {
	schema := testSchema(test)
	for _, scenario := range domainCases() {
		test.Run(scenario.name, func(test *testing.T) { checkDomainCase(test, schema, scenario) })
	}
	checkUnknownObservation(test, schema)
	if _, err := DecodeEnvelope(schema, []byte(`{"schema_version":"2.0"}`)); !IsPermanent(err) {
		test.Fatalf("malformed/unsupported envelope must fail permanently: %v", err)
	}
}

func domainPayload(test *testing.T, scenario domainCase) []byte {
	raw := example(test, "submission")
	raw = bytes.Replace(raw, []byte(`"24.90"`), []byte(`"`+scenario.amount+`"`), 1)
	raw = bytes.Replace(raw, []byte(`"27.90"`), []byte(`"`+scenario.reference+`"`), 1)
	raw = bytes.Replace(raw, []byte(`"gtin": null`), []byte(`"gtin": "`+scenario.gtin+`"`), 1)
	return bytes.Replace(raw, []byte(`"count": 1`), []byte(`"count": 1e0`), 1)
}

func checkDomainCase(test *testing.T, schema *jsonschema.Schema, scenario domainCase) {
	envelope, err := DecodeEnvelope(schema, domainPayload(test, scenario))
	requireNoError(test, err)
	observation, err := DecodeObservation(envelope.Observations[0])
	if (err == nil) != scenario.valid {
		test.Fatalf("valid=%t, error=%v", scenario.valid, err)
	}
	if err != nil && !IsPermanent(err) {
		test.Fatal("invalid domain input must be a permanent failure")
	}
	if scenario.valid && (*observation.Offers[0].Amount != scenario.amount || observation.Product.Package.Count != 1) {
		test.Fatalf("price or package changed: %+v", observation)
	}
}

func checkUnknownObservation(test *testing.T, schema *jsonschema.Schema) {
	envelope, err := DecodeEnvelope(schema, example(test, "unavailable"))
	requireNoError(test, err)
	observation, err := DecodeObservation(envelope.Observations[0])
	if err != nil || len(observation.Offers) != 0 || observation.Context.StoreID != nil {
		test.Fatalf("unknown values/empty offers changed: %+v, %v", observation, err)
	}
}

func expectedFingerprint(test *testing.T) string {
	raw, err := os.ReadFile("../../../../docs/contracts/ingestion/v1/fingerprints.json")
	requireNoError(test, err)
	var expected struct {
		Observation string `json:"first_observation_sha256"`
	}
	requireNoError(test, json.Unmarshal(raw, &expected))
	return expected.Observation
}

func reorderedPayload(test *testing.T, raw []byte) []byte {
	var document map[string]any
	requireNoError(test, json.Unmarshal(raw, &document))
	reordered, err := json.Marshal(document)
	requireNoError(test, err)
	reordered = bytes.Replace(reordered, []byte(`"count":1`), []byte(`"count":1.0`), 1)
	return bytes.ReplaceAll(reordered, []byte("São"), []byte(`S\u00e3o`))
}

func TestObservationFingerprintMatchesContract(test *testing.T) {
	schema := testSchema(test)
	raw := example(test, "submission")
	envelope, err := DecodeEnvelope(schema, raw)
	requireNoError(test, err)
	fingerprint := Fingerprint(envelope.ShopID, envelope.Observations[0])
	expected := expectedFingerprint(test)
	if hex.EncodeToString(fingerprint) != expected {
		test.Fatalf("fingerprint %x differs from contract %s", fingerprint, expected)
	}
	reordered, err := DecodeEnvelope(schema, reorderedPayload(test, raw))
	requireNoError(test, err)
	assertFingerprintSensitivity(test, fingerprint, reordered)
}

func assertFingerprintSensitivity(test *testing.T, fingerprint []byte, envelope Envelope) {
	observation := envelope.Observations[0]
	if !bytes.Equal(fingerprint, Fingerprint(envelope.ShopID, observation)) {
		test.Fatal("equivalent observation changed fingerprint")
	}
	if bytes.Equal(fingerprint, Fingerprint("another-shop", observation)) {
		test.Fatal("shop identity must affect fingerprint")
	}
	changed := bytes.Replace(observation, []byte(`"24.90"`), []byte(`"24.91"`), 1)
	if bytes.Equal(fingerprint, Fingerprint(envelope.ShopID, changed)) {
		test.Fatal("changed conditions/price must affect fingerprint")
	}
}

func TestCanonicalStrings(test *testing.T) {
	value := "<>&\u2028\u2029😀\"\\\b\t\n\f\r\x01 literal \\u2028"
	var output bytes.Buffer
	WriteCanonical(&output, value)
	if !strings.Contains(output.String(), "<>&\u2028\u2029😀") || !strings.Contains(output.String(), `\u0001`) {
		test.Fatalf("unexpected JCS encoding: %s", output.String())
	}
	var decoded string
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil || decoded != value {
		test.Fatalf("string was changed: %q, %v", decoded, err)
	}
}
