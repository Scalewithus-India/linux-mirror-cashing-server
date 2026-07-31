package mirror

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var rangeRE = regexp.MustCompile(`^bytes=(\d*)-(\d*)$`)

var (
	errInvalidRange      = errors.New("invalid range")
	errUnsatisfiableRange = errors.New("unsatisfiable")
)

// parseBytesRange returns inclusive start/end, or (-1,-1) for full body.
func parseBytesRange(header string, size int64) (start, end int64, err error) {
	if header == "" {
		return -1, -1, nil
	}
	header = strings.TrimSpace(header)
	if strings.Contains(header, ",") {
		return -1, -1, nil
	}
	m := rangeRE.FindStringSubmatch(header)
	if m == nil {
		return 0, 0, errInvalidRange
	}
	startS, endS := m[1], m[2]
	if startS == "" && endS == "" {
		return 0, 0, errInvalidRange
	}
	if startS == "" {
		length, e := strconv.ParseInt(endS, 10, 64)
		if e != nil || length <= 0 {
			return 0, 0, errInvalidRange
		}
		if size == 0 {
			return 0, 0, errUnsatisfiableRange
		}
		start = size - length
		if start < 0 {
			start = 0
		}
		end = size - 1
		return start, end, nil
	}
	start, e := strconv.ParseInt(startS, 10, 64)
	if e != nil {
		return 0, 0, errInvalidRange
	}
	if endS == "" {
		end = size - 1
	} else {
		end, e = strconv.ParseInt(endS, 10, 64)
		if e != nil {
			return 0, 0, errInvalidRange
		}
	}
	if size == 0 {
		return 0, 0, errUnsatisfiableRange
	}
	if start >= size {
		return 0, 0, errUnsatisfiableRange
	}
	if end >= size {
		end = size - 1
	}
	if start > end {
		return 0, 0, errInvalidRange
	}
	return start, end, nil
}
