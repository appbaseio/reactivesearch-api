package querytranslate

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// AddAdditionalFields will add new fields in the struct tag
// according to the ID of the struct type.
//
// As of now, this function will inject the following fields
// if available for the passed ID:
// - markdownDescription
func AddAdditionalFields(typePassed reflect.Type) []reflect.StructField {
	structFieldsToReturn := make([]reflect.StructField, 0)

	for index := 0; index < typePassed.NumField(); index++ {
		// Get the field
		fieldToWorkOn := typePassed.Field(index)

		// Extract the struct tag of the field,
		// Get the jsonschema_extras field and inject the markdownDescription.
		// Build a new struct tag field and add it to the current struct
		// field and finally add it to the final response array.
		tagOfField := fieldToWorkOn.Tag

		// Extract the ID of the field
		// The `json` value will be something like "id,omitempty".
		//
		// So, we can get it, split it by comma (,) and use the first index element
		// as the ID.
		jsonAsArr := regexp.MustCompile(`, ?`).Split(tagOfField.Get("json"), -1)

		IDToUse := ""

		if len(jsonAsArr) < 1 {
			// Use the lowercase'd name value as a fallback
			IDToUse = strings.ToLower(fieldToWorkOn.Name)
		} else {
			IDToUse = jsonAsArr[0]
		}

		updatedExtras := injectMarkdownDescription(tagOfField.Get("jsonschema_extras"), IDToUse)

		re := regexp.MustCompile(`jsonschema_extras:".*?"`)
		updatedTag := re.ReplaceAllString(string(tagOfField), fmt.Sprintf(`jsonschema_extras:"%s"`, updatedExtras))

		fieldToWorkOn.Tag = reflect.StructTag(updatedTag)

		structFieldsToReturn = append(structFieldsToReturn, fieldToWorkOn)
	}

	return structFieldsToReturn
}

// injectMarkdownDescription will inject the markdown description
// field to the jsonschema_extras field passed and return the
// modified string
func injectMarkdownDescription(extras string, ID string) string {
	// If the field is already present, no need to modify
	// the string.
	if strings.Contains(extras, "markdownDescription") {
		return extras
	}

	// Split the extras string based on comma
	// TODO: Remove whitespace after commas from the string

	splittedExtras := strings.Split(extras, ",")

	// Try to get the markdownDescription for the passed ID.
	mdDesc, isMdPresent := MARKDOWN_DESCRIPTIONS[ID]

	// If no description is present for the passed ID
	if !isMdPresent {
		return extras
	}

	// Inject markdownDescription
	splittedExtras = append(splittedExtras, fmt.Sprintf("markdownDescription=%s", mdDesc))

	return strings.Join(splittedExtras, ",")
}

var MARKDOWN_DESCRIPTIONS = map[string]string{
	"id": "The unique identifier for the query can be referenced in the `react` property of other queries. The response of the `ReactiveSearch API` is a map of query ids to `Elasticsearch` response which means that `id` is also useful to retrieve the response for a particular query.",
}
