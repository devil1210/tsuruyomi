package models

import (
	"strconv"
	"strings"
)

type IncludeOrExclude int

const (
	Exclude IncludeOrExclude = 0
	Include IncludeOrExclude = 1
	Unset   IncludeOrExclude = -1
)

// GraphQLEnum returns the uppercase GraphQL enum string ("INCLUDE", "EXCLUDE", "UNSET").
func (i IncludeOrExclude) GraphQLEnum() string {
	switch i {
	case Include:
		return "INCLUDE"
	case Exclude:
		return "EXCLUDE"
	default:
		return "UNSET"
	}
}

// ParseIncludeOrExclude parses string, int, float, or bool into IncludeOrExclude.
func ParseIncludeOrExclude(val interface{}) IncludeOrExclude {
	if val == nil {
		return Unset
	}
	switch v := val.(type) {
	case string:
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "INCLUDE":
			return Include
		case "EXCLUDE":
			return Exclude
		case "UNSET":
			return Unset
		default:
			if b, err := strconv.ParseBool(v); err == nil {
				if b {
					return Include
				}
				return Exclude
			}
			if n, err := strconv.Atoi(v); err == nil {
				return IncludeOrExclude(n)
			}
		}
	case bool:
		if v {
			return Include
		}
		return Exclude
	case int:
		return IncludeOrExclude(v)
	case int64:
		return IncludeOrExclude(int(v))
	case float64:
		return IncludeOrExclude(int(v))
	}
	return Unset
}


type CategoryDataClass struct {
	ID                int              `json:"id"`
	Order             int              `json:"order"`
	Name              string           `json:"name"`
	Default           bool             `json:"default"`
	IncludeInUpdate   IncludeOrExclude `json:"includeInUpdate"`
	IncludeInDownload IncludeOrExclude `json:"includeInDownload"`
	Version           int64            `json:"version"`
	UID               int64            `json:"uid"`
	LastModifiedAt    int64            `json:"lastModifiedAt"`
	Size              int              `json:"size"`
	Meta              map[string]string `json:"meta"`
}
