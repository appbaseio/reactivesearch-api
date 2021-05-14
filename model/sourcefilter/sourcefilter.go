package sourcefilter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/common/log"
)

const logTag = "[sourcefiltering]"

// To filter the source based on the include and exclude patterns
func ApplySourceFiltering(source map[string]interface{}, include []string, exclude []string) interface{} {
	// Avoid calculation if source filters are not defined
	if len(include) == 0 && len(exclude) == 0 {
		return source
	}
	var filteredSource = make(map[string]interface{})
	// Convert map to dot notation
	mapWithDotNotationKeys := dotNotate(source, nil, "")
	for key, v := range mapWithDotNotationKeys {
		isValidKey := true
		// Remove numbers from key to match the pattern
		// "a.0.b.1.c" would become "a.b.c"
		// "a.0.b.1.c.2" would become "a.b.c.2"
		re := regexp.MustCompile(`\.[0-9]+\.`)
		keyToCompare := re.ReplaceAllString(key, ".")
		// Check if key is valid
		for _, pattern := range include {
			// If pattern matches then avoid further iterations
			pattern = strings.Replace(pattern, "*", ".*", -1)
			matched, err := regexp.MatchString(pattern, keyToCompare)
			if err != nil {
				log.Errorln(logTag, ":", err)
			} else if matched {
				isValidKey = true
				break
			} else {
				isValidKey = false
			}
		}

		for _, pattern := range exclude {
			// If pattern matches then avoid further iterations
			pattern = strings.Replace(pattern, "*", ".*", -1)
			matched, err := regexp.MatchString(pattern, keyToCompare)
			if err != nil {
				log.Errorln(logTag, ":", err)
			} else if matched {
				isValidKey = false
				break
			}
		}
		if isValidKey {
			filteredSource[key] = v
		}
	}
	// Convert dot notated map to map representation
	return dotNotationToMap(filteredSource)
}

// Converts a map to dot representation
// For examples,
// Example 1
// Input:
// map[string]interface{}{
// 	"key1": "value1",
// 	"key2": map[string]interface{}{
// 		"key21": "value21",
// 		"key22": map[string]interface{}{
// 			"key221": "value221",
// 			"key222": "value222",
// 		},
// 	},
// }
// Output:
// map[string]interface{}{
// 	"key1":              "value1",
// 	"key2.key21":        "value21",
// 	"key2.key22.key221": "value221",
// 	"key2.key22.key222": "value222",
// }
//
// Example 2 => with array
// Input:
// map[string]interface{}{
// 	"key1": []interface{}{"value11", "value12", "value13"},
// }
// Output:
// map[string]interface{}{
// 	"key1.0": "value11",
// 	"key1.1": "value12",
// 	"key1.2": "value13",
// }
func dotNotate(source interface{}, target map[string]interface{}, prefix string) map[string]interface{} {
	if target == nil {
		target = make(map[string]interface{})
	}
	// handle map values
	sourceAsMap, ok := source.(map[string]interface{})
	if ok {
		for k, v := range sourceAsMap {
			valueAsMap, ok := v.(map[string]interface{})
			if ok {
				dotNotate(valueAsMap, target, prefix+k+".")
			} else {
				valueAsArray, ok := v.([]interface{})
				if ok {
					dotNotate(valueAsArray, target, prefix+k)
				} else {
					target[prefix+k] = v
				}
			}
		}
	}
	// handle array values
	sourceAsArray, ok := source.([]interface{})
	if ok {
		for k, v := range sourceAsArray {
			valueAsMap, ok := v.(map[string]interface{})
			keyAsString := strconv.Itoa(k)
			effectiveKey := prefix + "." + keyAsString
			if ok {
				dotNotate(valueAsMap, target, effectiveKey+".")
			} else {
				valueAsArray, ok := v.([]interface{})
				if ok {
					dotNotate(valueAsArray, target, effectiveKey)
				} else {
					target[effectiveKey] = v
				}
			}
		}
	}
	return target
}

// Helper method to generate the nested map for a particular key represented by dot notation
// It would find the correct path for the value in the existing map
// For example,
// Example 1 => key path for nested map
// generateMap(map[string]interface{"key1": "val1"}, "key2.key21", "val21")
// Output => map[string]interface{"key1": "val1", "key2": map[string]interface{}{"key21": "val21"}}
//
// Example 2 => key path with array
// generateMap(map[string]interface{"key1": []interface{"val1"}}, "key1.1", "val2")
// Output => map[string]interface{"key1": []interface{"val1", "val2"}}
func generateMap(source interface{}, key string, value interface{}) interface{} {
	var output = source
	tokens := strings.Split(key, ".")
	if len(tokens) > 1 {
		nextKey := strings.SplitN(key, ".", 2)[1]
		if output == nil {
			_, err := strconv.Atoi(tokens[0])
			if err != nil {
				// in case of string key initialize output as map
				output = make(map[string]interface{})
			} else {
				// if key is an integer then
				output = make([]interface{}, 0)
			}
		}
		outputAsMap, ok := output.(map[string]interface{})
		if ok {
			outputAsMap[tokens[0]] = generateMap(
				outputAsMap[tokens[0]],
				nextKey,
				value,
			)
			output = outputAsMap
		} else {
			outputAsArray, ok := output.([]interface{})
			if ok {
				index, err := strconv.Atoi(tokens[0])
				if err != nil {
					log.Errorln(logTag, ":", err)
				} else {
					// Make space for new element
					for len(outputAsArray) < index+1 {
						outputAsArray = append(outputAsArray, nil)
					}
					outputAsArray[index] = generateMap(
						outputAsArray[index],
						nextKey,
						value,
					)
					output = outputAsArray
				}
			}
		}
	} else {
		if output == nil {
			_, err := strconv.Atoi(key)
			if err != nil {
				// in case of string key initialize output as map
				output = make(map[string]interface{})
			} else {
				// if key is an integer then
				output = make([]interface{}, 0)
			}
		}
		outputAsMap, ok := output.(map[string]interface{})
		if ok {
			outputAsMap[key] = value
			output = outputAsMap
		} else {
			outputAsArray, ok := output.([]interface{})
			if ok {
				index, err := strconv.Atoi(key)
				if err != nil {
					log.Errorln(logTag, ":", err)
				} else {
					// Make space for new element
					for len(outputAsArray) < index+1 {
						outputAsArray = append(outputAsArray, nil)
					}
					outputAsArray[index] = value
					output = outputAsArray
				}
			}
		}
	}
	return output
}

// Converts the dot representation of map to actual map
// For example,
// Input: {
// 	"key1":              "value1",
// 	"key2.key21":        "value21",
// 	"key2.key22.key221": "value221",
// 	"key2.key22.key222": "value222",
//    "key3.0":            "value31",
//    "key3.0":            "value32",
// }

// Output: map[string]interface{}{
// 	"key1": "value1",
// 	"key2": map[string]interface{}{
// 		"key21": "value21",
// 		"key22": map[string]interface{}{
// 			"key221": "value221",
// 			"key222": "value222",
// 		},
// 	},
//    "key3": []interface{}{"value31", "value32"}
// }
func dotNotationToMap(source map[string]interface{}) interface{} {
	var output interface{}
	for k := range source {
		output = generateMap(output, k, source[k])
	}
	return output
}
