package models

type IncludeOrExclude int

const (
	Exclude IncludeOrExclude = 0
	Include IncludeOrExclude = 1
	Unset   IncludeOrExclude = -1
)

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
