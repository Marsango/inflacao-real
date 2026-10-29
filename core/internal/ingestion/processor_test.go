package ingestion

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGTINCheckDigits(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"96385074", true}, {"036000291452", true}, {"4006381333931", true},
		{"10012345000017", true}, {"4006381333932", false}, {"96385075", false},
		{"036000291453", false}, {"10012345000018", false}, {"123", false},
		{"400638133393x", false}, {"", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if got := validGTIN(tc.value); got != tc.valid {
				t.Fatalf("validGTIN(%q)=%t, want %t", tc.value, got, tc.valid)
			}
		})
	}
}

func TestWorkerDomainValidationAndExactPrices(t *testing.T) {
	schema, err := compileSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, amount, reference, gtin string
		valid                         bool
	}{
		{"maximum", "999999999.99", "999999999.99", "4006381333931", true},
		{"zero", "0.00", "0.00", "036000291452", true},
		{"one cent below", "999999999.99", "999999999.98", "4006381333931", false},
		{"smaller reference", "24.90", "9.99", "4006381333931", false},
		{"bad GTIN", "24.90", "27.90", "4006381333932", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := example(t, "submission")
			raw = bytes.Replace(raw, []byte(`"24.90"`), []byte(`"`+tc.amount+`"`), 1)
			raw = bytes.Replace(raw, []byte(`"27.90"`), []byte(`"`+tc.reference+`"`), 1)
			raw = bytes.Replace(raw, []byte(`"gtin": null`), []byte(`"gtin": "`+tc.gtin+`"`), 1)
			// JSON Schema accepts integer-valued exponent/decimal representations.
			raw = bytes.Replace(raw, []byte(`"count": 1`), []byte(`"count": 1e0`), 1)
			e, err := decodeEnvelope(schema, raw)
			if err != nil {
				t.Fatal(err)
			}
			o, err := decodeObservation(e.Observations[0])
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
			if err != nil && !isPermanent(err) {
				t.Fatal("invalid domain input must be a permanent failure")
			}
			if tc.valid && (*o.Offers[0].Amount != tc.amount || o.Product.Package.Count != 1) {
				t.Fatalf("price or package changed: %+v", o)
			}
		})
	}
	e, err := decodeEnvelope(schema, example(t, "unavailable"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := decodeObservation(e.Observations[0])
	if err != nil || len(o.Offers) != 0 || o.Context.StoreID != nil {
		t.Fatalf("unknown values/empty offers changed: %+v, %v", o, err)
	}
	if _, err := decodeEnvelope(schema, []byte(`{"schema_version":"2.0"}`)); !isPermanent(err) {
		t.Fatalf("malformed/unsupported envelope must fail permanently: %v", err)
	}
}

func TestObservationFingerprintMatchesContract(t *testing.T) {
	schema, err := compileSchema()
	if err != nil {
		t.Fatal(err)
	}
	raw := example(t, "submission")
	e, err := decodeEnvelope(schema, raw)
	if err != nil {
		t.Fatal(err)
	}
	vector, err := os.ReadFile("../../../docs/contracts/ingestion/v1/fingerprints.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Observation string `json:"first_observation_sha256"`
	}
	if err := json.Unmarshal(vector, &expected); err != nil {
		t.Fatal(err)
	}
	hash := observationFingerprint(e.ShopID, e.Observations[0])
	if hex.EncodeToString(hash) != expected.Observation {
		t.Fatalf("fingerprint %x differs from contract %s", hash, expected.Observation)
	}
	// Equivalent escapes, key order, whitespace and integer spellings are ignored.
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	reordered = bytes.Replace(reordered, []byte(`"count":1`), []byte(`"count":1.0`), 1)
	reordered = bytes.ReplaceAll(reordered, []byte("São"), []byte(`S\u00e3o`))
	e2, err := decodeEnvelope(schema, reordered)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(hash, observationFingerprint(e2.ShopID, e2.Observations[0])) {
		t.Fatal("equivalent observation changed fingerprint")
	}
	if bytes.Equal(hash, observationFingerprint("another-shop", e2.Observations[0])) {
		t.Fatal("shop identity must affect fingerprint")
	}
	changed := bytes.Replace(e2.Observations[0], []byte(`"24.90"`), []byte(`"24.91"`), 1)
	if bytes.Equal(hash, observationFingerprint(e2.ShopID, changed)) {
		t.Fatal("changed conditions/price must affect fingerprint")
	}
}

func TestCanonicalStrings(t *testing.T) {
	value := "<>&\u2028\u2029😀\"\\\b\t\n\f\r\x01 literal \\u2028"
	var out bytes.Buffer
	writeCanonical(&out, value)
	if !strings.Contains(out.String(), "<>&\u2028\u2029😀") || !strings.Contains(out.String(), `\u0001`) {
		t.Fatalf("unexpected JCS encoding: %s", out.String())
	}
	var decoded string
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || decoded != value {
		t.Fatalf("string was changed: %q, %v", decoded, err)
	}
}
