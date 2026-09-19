package assetpaths

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedAssetPathCases(t *testing.T) {
	data, err := os.ReadFile("../../Tests/fixtures/asset-paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, From, To, Source, Expected string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases {
		t.Run(item.Name, func(t *testing.T) {
			if actual := Rewrite(item.Source, item.From, item.To); actual != item.Expected {
				t.Fatalf("got %q, want %q", actual, item.Expected)
			}
		})
	}
}
