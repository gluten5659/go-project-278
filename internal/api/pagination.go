package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

const rangeBoundsCount = 2

var errMalformedRange = errors.New("range must be [first, last] with 0 <= first <= last")

type pageRange struct {
	firstIndex int64
	lastIndex  int64
}

func (bounds pageRange) contentRange(resource string, totalRecords int64) string {
	return fmt.Sprintf("%s %d-%d/%d", resource, bounds.firstIndex, bounds.lastIndex, totalRecords)
}

func (bounds pageRange) pageSize() int64 {
	return bounds.lastIndex - bounds.firstIndex
}

func parsePageRange(rawRange string, totalRecords int64) (pageRange, error) {
	if rawRange == "" {
		return pageRange{firstIndex: 0, lastIndex: totalRecords}, nil
	}

	var bounds []int64

	err := json.Unmarshal([]byte(rawRange), &bounds)
	if err != nil {
		return pageRange{}, fmt.Errorf("json.Unmarshal: %w", err)
	}

	if len(bounds) != rangeBoundsCount {
		return pageRange{}, errMalformedRange
	}

	parsed := pageRange{firstIndex: bounds[0], lastIndex: bounds[1]}

	if parsed.firstIndex < 0 || parsed.lastIndex < parsed.firstIndex {
		return pageRange{}, errMalformedRange
	}

	return parsed, nil
}
