package models

type SourceDataClass struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Lang           string  `json:"lang"`
	IconURL        string  `json:"iconUrl"`
	SupportsLatest bool    `json:"supportsLatest"`
	IsConfigurable bool    `json:"isConfigurable"`
	IsNsfw         bool    `json:"isNsfw"`
	DisplayName    string  `json:"displayName"`
	BaseURL        *string `json:"baseUrl"`
	PkgName        string  `json:"pkgName"`
	ContentWarning string  `json:"contentWarning"`
}

type SourcePreferenceChange struct {
	Position int    `json:"position"`
	Value    string `json:"value"`
}

type PreferenceObject struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Value       string   `json:"value"`
	Default     string   `json:"default"`
	Entries     []string `json:"entries,omitempty"`
	EntryValues []string `json:"entryValues,omitempty"`
}

type FilterObject struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Additional dynamic Tachiyomi filters mapping properties can go here
}

type FilterChange struct {
	Position int         `json:"position"`
	Value    interface{} `json:"value"`
}

type FilterData struct {
	// Compound filtering input payload matching Tachiyomi search parameters
}
