package scraperhealth

import (
	"bytes"
	_ "embed"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:generate cp ../../../docs/contracts/scraper-reports/v1/schema.json schema.json
//go:embed schema.json
var schemaJSON []byte

var reportSchema = compileSchema()

func compileSchema() *jsonschema.Schema {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		panic(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	const location = "urn:nossa-inflacao:scraper-reports:v1"
	if err = compiler.AddResource(location, document); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		panic(err)
	}
	return schema
}
func validateSchema(body []byte) error {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return err
	}
	return reportSchema.Validate(document)
}
