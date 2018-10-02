package category

import (
	"encoding/json"
	"fmt"
)

type Category int

const (
	Cat Category = iota
	Bulk
	Search
	Cluster
	Remote
	Scripts
	Doc
	Count
	Create
	Source
	FieldCaps
	Explain
	Analyze
	DeleteByQuery
	Close
	Alias
	Aliases
	Template
	Cache
	Mapping
	Flush
	Forcemerge
	Settings
	Upgrade
	Mappings
	Open
	Refresh
	Recovery
	Segments
	Shrink
	ShardStores
	Rollover
	Split
	Stats
	Ingest
	Validate
	Msearch
	Nodes
	Mget
	Mtermvectors
	RankEval
	Reindex
	UpdateByQuery
	Render
	SearchShards
	Snapshot
	Tasks
	Termvectors
	Update
)

const _CategoryName = "catbulksearchclusterremotescriptsdoccountcreatesourcefieldcapsexplain" +
	"analyzedeletebyqueryclosealiasaliasestemplatecachemappingflushforcemergesettings" +
	"upgrademappingsopenrefreshrecoverysegmentsshrinkshardstoresrolloversplit" +
	"statsingestvalidatemsearchnodesmgetmtermvectorsrankevalreindexupdatebyquery" +
	"rendersearchshardssnapshottaskstermvectorsupdate"

var _CategoryIndex = [...]uint16{
	0, 3, 7, 13, 20, 26, 33, 36, 41, 47,
	53, 62, 69, 76, 89, 94, 99, 106, 114,
	119, 126, 131, 141, 149, 156, 164, 168,
	175, 183, 191, 197, 208, 216, 221, 226,
	232, 240, 247, 252, 256, 268, 276, 283,
	296, 302, 314, 322, 327, 338, 344,
}

func (c Category) String() string {
	if c < 0 || c >= Category(len(_CategoryIndex)-1) {
		return fmt.Sprintf("Category(%d)", c)
	}
	return _CategoryName[_CategoryIndex[c]:_CategoryIndex[c+1]]
}

var _CategoryValues = []Category{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 44, 45, 46, 47, 48,
}

var _CategoryNameToValueMap = map[string]Category{
	_CategoryName[0:3]:     0,
	_CategoryName[3:7]:     1,
	_CategoryName[7:13]:    2,
	_CategoryName[13:20]:   3,
	_CategoryName[20:26]:   4,
	_CategoryName[26:33]:   5,
	_CategoryName[33:36]:   6,
	_CategoryName[36:41]:   7,
	_CategoryName[41:47]:   8,
	_CategoryName[47:53]:   9,
	_CategoryName[53:62]:   10,
	_CategoryName[62:69]:   11,
	_CategoryName[69:76]:   12,
	_CategoryName[76:89]:   13,
	_CategoryName[89:94]:   14,
	_CategoryName[94:99]:   15,
	_CategoryName[99:106]:  16,
	_CategoryName[106:114]: 17,
	_CategoryName[114:119]: 18,
	_CategoryName[119:126]: 19,
	_CategoryName[126:131]: 20,
	_CategoryName[131:141]: 21,
	_CategoryName[141:149]: 22,
	_CategoryName[149:156]: 23,
	_CategoryName[156:164]: 24,
	_CategoryName[164:168]: 25,
	_CategoryName[168:175]: 26,
	_CategoryName[175:183]: 27,
	_CategoryName[183:191]: 28,
	_CategoryName[191:197]: 29,
	_CategoryName[197:208]: 30,
	_CategoryName[208:216]: 31,
	_CategoryName[216:221]: 32,
	_CategoryName[221:226]: 33,
	_CategoryName[226:232]: 34,
	_CategoryName[232:240]: 35,
	_CategoryName[240:247]: 36,
	_CategoryName[247:252]: 37,
	_CategoryName[252:256]: 38,
	_CategoryName[256:268]: 39,
	_CategoryName[268:276]: 40,
	_CategoryName[276:283]: 41,
	_CategoryName[283:296]: 42,
	_CategoryName[296:302]: 43,
	_CategoryName[302:314]: 44,
	_CategoryName[314:322]: 45,
	_CategoryName[322:327]: 46,
	_CategoryName[327:338]: 47,
	_CategoryName[338:344]: 48,
}

// CategoryString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func Value(s string) (Category, error) {
	if val, ok := _CategoryNameToValueMap[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to Category values", s)
}

// CategoryValues returns all values of the enum
func Values() []Category {
	return _CategoryValues
}

// IsACategory returns "true" if the value is listed in the enum definition. "false" otherwise
func (c Category) IsACategory() bool {
	for _, v := range _CategoryValues {
		if c == v {
			return true
		}
	}
	return false
}

// MarshalJSON implements the json.Marshaler interface for Category
func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface for Category
func (c *Category) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("category should be a string, got %s", data)
	}

	var err error
	*c, err = Value(s)
	return err
}
