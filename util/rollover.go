package util

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// RolloverConditions mirrors the rollover API condition fields evaluated
// client-side on Elasticsearch Serverless, where conditions cannot be sent
// in the rollover request body.
type RolloverConditions struct {
	MaxAge  string
	MaxDocs int
	MaxSize string
}

// RolloverConditionsFromMap extracts rollover thresholds from the JSON-shaped
// map used by rollover jobs (max_age, max_docs, max_size).
func RolloverConditionsFromMap(conditions map[string]interface{}) RolloverConditions {
	var parsed RolloverConditions
	if v, ok := conditions["max_age"].(string); ok {
		parsed.MaxAge = v
	}
	if v, ok := conditions["max_docs"].(float64); ok {
		parsed.MaxDocs = int(v)
	}
	if v, ok := conditions["max_size"].(string); ok {
		parsed.MaxSize = v
	}
	return parsed
}

// WriteIndexMeetsRolloverConditions reports whether the alias write index
// satisfies any rollover threshold. Elasticsearch rollover uses OR semantics:
// rollover happens when max_age OR max_docs OR max_size is met.
func WriteIndexMeetsRolloverConditions(ctx context.Context, alias string, conditions map[string]interface{}) (bool, error) {
	parsed := RolloverConditionsFromMap(conditions)
	writeIndex, err := GetWriteIndexForAlias(ctx, alias)
	if err != nil {
		return false, err
	}

	catRows, err := GetClient7().CatIndices().Index(writeIndex).Do(ctx)
	if err != nil {
		return false, fmt.Errorf("fetching index stats for %s: %w", writeIndex, err)
	}
	if len(catRows) == 0 {
		return false, fmt.Errorf("index %s not found", writeIndex)
	}
	row := catRows[0]

	if parsed.MaxAge != "" {
		maxAge, err := parseESDuration(parsed.MaxAge)
		if err != nil {
			return false, fmt.Errorf("parsing max_age %q: %w", parsed.MaxAge, err)
		}
		indexAge := time.Since(time.Unix(0, row.CreationDate*int64(time.Millisecond)))
		if indexAge >= maxAge {
			log.Debugf("rollover condition met for %s: max_age (%s >= %s)", writeIndex, indexAge, maxAge)
			return true, nil
		}
	}

	if parsed.MaxDocs > 0 && int64(row.DocsCount) >= int64(parsed.MaxDocs) {
		log.Debugf("rollover condition met for %s: max_docs (%d >= %d)", writeIndex, row.DocsCount, parsed.MaxDocs)
		return true, nil
	}

	if parsed.MaxSize != "" {
		maxBytes, err := parseStoreSize(parsed.MaxSize)
		if err != nil {
			return false, fmt.Errorf("parsing max_size %q: %w", parsed.MaxSize, err)
		}
		storeSize := row.PriStoreSize
		if storeSize == "" || storeSize == "-" {
			storeSize = row.StoreSize
		}
		indexBytes, err := parseStoreSize(storeSize)
		if err != nil {
			return false, fmt.Errorf("parsing store size for %s: %w", writeIndex, err)
		}
		if indexBytes >= maxBytes {
			log.Debugf("rollover condition met for %s: max_size (%d >= %d)", writeIndex, indexBytes, maxBytes)
			return true, nil
		}
	}

	return false, nil
}

// GetWriteIndexForAlias returns the current write index backing an alias.
func GetWriteIndexForAlias(ctx context.Context, alias string) (string, error) {
	aliasesResult, err := GetClient7().Aliases().Alias(alias).Do(ctx)
	if err != nil {
		return "", err
	}

	var fallback string
	for indexName, indexInfo := range aliasesResult.Indices {
		for _, aliasInfo := range indexInfo.Aliases {
			if aliasInfo.AliasName != alias {
				continue
			}
			if aliasInfo.IsWriteIndex {
				return indexName, nil
			}
			if fallback == "" {
				fallback = indexName
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no write index found for alias %s", alias)
}

// parseESDuration converts Elasticsearch time values (e.g. "7d", "30d";
// units: ms/s/m/h/d/w) into Go durations.
func parseESDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}

	units := []struct {
		suffix string
		scale  time.Duration
	}{
		{"ms", time.Millisecond},
		{"w", 7 * 24 * time.Hour},
		{"d", 24 * time.Hour},
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			if number == "" {
				return 0, fmt.Errorf("invalid duration %q", value)
			}
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q: %w", value, err)
			}
			return time.Duration(parsed * float64(unit.scale)), nil
		}
	}
	return 0, fmt.Errorf("unsupported duration %q", value)
}

// parseStoreSize converts Elasticsearch size strings (e.g. "1gb", "10gb") or
// plain byte counts to bytes. A dash or empty value is treated as zero.
func parseStoreSize(size string) (int64, error) {
	size = strings.TrimSpace(strings.ToLower(size))
	if size == "" || size == "-" {
		return 0, nil
	}

	if numeric, err := strconv.ParseInt(size, 10, 64); err == nil {
		return numeric, nil
	}

	multiplier := int64(1)
	units := []struct {
		suffix string
		mult   int64
	}{
		{"pb", 1024 * 1024 * 1024 * 1024 * 1024},
		{"tb", 1024 * 1024 * 1024 * 1024},
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(size, unit.suffix) {
			size = strings.TrimSuffix(size, unit.suffix)
			multiplier = unit.mult
			break
		}
	}

	size = strings.TrimSpace(size)
	if size == "" {
		return 0, fmt.Errorf("invalid store size")
	}

	value, err := strconv.ParseFloat(size, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid store size %q: %w", size, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("invalid store size %q", size)
	}

	return int64(value * float64(multiplier)), nil
}

// NewIndicesRolloverService builds a rollover request for the target cluster.
// On Elasticsearch Serverless, conditions are omitted because the API rejects
// them; callers must evaluate thresholds client-side before invoking Do.
func NewIndicesRolloverService(alias string, conditions map[string]interface{}) *es7.IndicesRolloverService {
	svc := es7.NewIndicesRolloverService(GetClient7()).Alias(alias)
	if !IsServerless() && len(conditions) > 0 {
		svc = svc.Conditions(conditions)
	}
	return svc
}
