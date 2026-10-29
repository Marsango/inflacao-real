package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type permanentError struct{ error }

func Permanent(format string, arguments ...any) error {
	return permanentError{fmt.Errorf(format, arguments...)}
}

func IsPermanent(err error) bool {
	var inputError permanentError
	return errors.As(err, &inputError)
}

func DecodeEnvelope(schema *jsonschema.Schema, raw []byte) (Envelope, error) {
	if _, err := ValidatePayload(schema, raw); err != nil {
		return Envelope{}, Permanent("invalid stored payload: %w", err)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return Envelope{}, Permanent("decode payload: %w", err)
	}
	var canonical bytes.Buffer
	WriteCanonical(&canonical, document)
	var envelope Envelope
	if err := json.Unmarshal(canonical.Bytes(), &envelope); err != nil {
		return Envelope{}, Permanent("decode envelope: %w", err)
	}
	return envelope, nil
}

func DecodeObservation(raw []byte) (Observation, error) {
	var observation Observation
	if err := json.Unmarshal(raw, &observation); err != nil {
		return observation, Permanent("decode observation: %w", err)
	}
	if err := observation.validate(); err != nil {
		return observation, err
	}
	return observation, nil
}

func (observation Observation) validate() error {
	if observation.Product.GTIN != nil && !ValidGTIN(*observation.Product.GTIN) {
		return Permanent("observation %s: invalid GTIN check digit", observation.ID)
	}
	var err error
	for position := 0; position < len(observation.Offers) && err == nil; position++ {
		err = observation.Offers[position].validateReference(observation.ID)
	}
	return err
}

func (offer Offer) validateReference(observationID string) error {
	if offer.ReferenceAmount != nil && moneyMinor(*offer.ReferenceAmount) < moneyMinor(*offer.Amount) {
		return Permanent("observation %s: reference amount is less than price", observationID)
	}
	return nil
}

func ValidGTIN(value string) bool {
	lengths := map[int]bool{8: true, 12: true, 13: true, 14: true}
	if !lengths[len(value)] || strings.Trim(value, "0123456789") != "" {
		return false
	}
	sum, weight := 0, 1
	for position := len(value) - 1; position >= 0; position-- {
		sum += int(value[position]-'0') * weight
		weight = 4 - weight
	}
	return sum%10 == 0
}

func moneyMinor(value string) int64 {
	amount, _ := strconv.ParseInt(strings.ReplaceAll(value, ".", ""), 10, 64)
	return amount
}
