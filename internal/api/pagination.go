package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	rangeBoundsCount = 2

	maxPageSize = 1000
)

var errMalformedRange = errors.New("range must be [first, last] with 0 <= first <= last")

type pageRange struct {
	offset int64
	size   int64
}

func (bounds pageRange) contentRange(resource string, recordCount, totalRecords int64) string {
	if recordCount == 0 {
		return fmt.Sprintf("%s */%d", resource, totalRecords)
	}

	lastIndex := bounds.offset + recordCount - 1

	return fmt.Sprintf("%s %d-%d/%d", resource, bounds.offset, lastIndex, totalRecords)
}

func parsePageRange(rawRange string, totalRecords int64) (pageRange, error) {
	if rawRange == "" {
		return pageRange{offset: 0, size: totalRecords}, nil
	}

	var bounds []int64

	err := json.Unmarshal([]byte(rawRange), &bounds)
	if err != nil {
		return pageRange{}, fmt.Errorf("parse range %q: %w", rawRange, err)
	}

	if len(bounds) != rangeBoundsCount {
		return pageRange{}, errMalformedRange
	}

	firstIndex, lastIndex := bounds[0], bounds[1]

	if firstIndex < 0 || lastIndex < firstIndex {
		return pageRange{}, errMalformedRange
	}

	indexSpan := lastIndex - firstIndex

	return pageRange{offset: firstIndex, size: min(indexSpan, maxPageSize-1) + 1}, nil
}
