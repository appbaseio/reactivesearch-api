package synonyms

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SynonymType int

const (
	OneWay SynonymType = iota // after
	Equivalent
	IndexPattern = "__index__"
)

func (o *SynonymType) UnmarshalJSON(bytes []byte) error {
	var synonymType string
	err := json.Unmarshal(bytes, &synonymType)
	if err != nil {
		return err
	}
	switch synonymType {
	case OneWay.String():
		*o = OneWay
	case Equivalent.String():
		*o = Equivalent
	default:
		return fmt.Errorf("invalid synonymType encountered: %v", synonymType)
	}
	return nil
}

func (o SynonymType) MarshalJSON() ([]byte, error) {
	var synonymType string
	switch o {
	case OneWay:
		synonymType = OneWay.String()
	case Equivalent:
		synonymType = Equivalent.String()
	default:
		return nil, fmt.Errorf("invalid synonymType encountered: %v", o)
	}
	return json.Marshal(synonymType)
}

// String is the implementation of Stringer interface that returns the string representation of ActionType type.
func (o SynonymType) String() string {
	return [...]string{
		"one-way",
		"equivalent",
	}[o]
}

type BaseSynonymStruct struct {
	Type    SynonymType `json:"type"`
	Synonym string      `json:"synonym"`
	Index   string      `json:"index"`
}

// SynonymsStruct struct for settings request
type SynonymsStruct struct {
	Id string `json:"_id"`
	BaseSynonymStruct
}

// AppendIndexToSynonymID will append the index name to the synonym's
// ID.
func AppendIndexToSynonymID(ID string, index string) string {
	return ID + IndexPattern + index
}

// RemoveIndexFromSynonymID will remove the index name from the synonym's
// ID
//
// This function will return the raw ID and the index
func RemoveIndexFromSynonymID(ID string) (string, string) {
	splittedID := strings.Split(ID, IndexPattern)
	if len(splittedID) != 2 {
		return ID, ""
	}

	return splittedID[0], splittedID[1]
}
