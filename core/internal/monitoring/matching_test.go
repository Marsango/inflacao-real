package monitoring

import (
	"encoding/json"
	"os"
	"testing"
)

type matchingExample struct {
	Target Target `json:"target"`
	Cases  []struct {
		Name    string `json:"name"`
		Matches bool   `json:"matches"`
	} `json:"cases"`
}

func TestSharedMatchingExamples(t *testing.T) {
	data, err := os.ReadFile("../../../docs/contracts/monitoring/v1/matching-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples []matchingExample
	if err := json.Unmarshal(data, &examples); err != nil {
		t.Fatal(err)
	}
	for _, example := range examples {
		checkMatchingExample(t, example)
	}
}

func checkMatchingExample(t *testing.T, example matchingExample) {
	t.Helper()
	for _, candidate := range example.Cases {
		checkMatch(t, example.Target, candidate.Name, candidate.Matches)
	}
}

func checkMatch(t *testing.T, target Target, name string, expected bool) {
	t.Helper()
	if actual := target.Matches(name); actual != expected {
		t.Errorf("target=%s name=%q: matched=%t; want %t", target.ID, name, actual, expected)
	}
}
