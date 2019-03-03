package reindexer

import (
	"reflect"
	"testing"
)

var reindexedNamesTest = []struct {
	index    string
	expected string
	err      string
}{
	{
		"twitter",
		"twitter_reindexed_1",
		"",
	},
	{
		"twitter_reindexed_2",
		"twitter_reindexed_3",
		"",
	},
	{
		"twitter_reindexed_1@",
		"",
		"strconv.Atoi: parsing \"1@\": invalid syntax",
	},
	// TODO: add a test case for invalid regex compilation error if possible
}

func TestReindexedName(t *testing.T) {
	for _, tt := range reindexedNamesTest {
		actual, err := reindexedName(tt.index)

		if !reflect.DeepEqual(actual, tt.expected) {
			t.Fatalf("Reindexed name mismatch, expected: %s got: %s\n", tt.expected, actual)
		}

		if !compareErrs(tt.err, err) {
			t.Fatalf("Reindexed name error mismatch, expected: %s got: %s\n", tt.err, err)
		}
	}
}
