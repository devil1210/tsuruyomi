package models

type ExtensionDataClass struct {
	Repo        *string `json:"repo"`
	ApkName     string  `json:"apkName"`
	IconURL     string  `json:"iconUrl"`
	Name        string  `json:"name"`
	PkgName     string  `json:"pkgName"`
	VersionName string  `json:"versionName"`
	VersionCode int     `json:"versionCode"`
	Lang        string  `json:"lang"`
	IsNsfw      bool    `json:"isNsfw"`
	Installed   bool    `json:"installed"`
	HasUpdate   bool    `json:"hasUpdate"`
	Obsolete    bool    `json:"obsolete"`
}

type ExtensionStore struct {
	ID               int    `json:"id"`
	IndexURL         string `json:"indexUrl"`
	Name             string `json:"name"`
	BadgeLabel       string `json:"badgeLabel"`
	SigningKey       string `json:"signingKey"`
	ContactWebsite   string `json:"contactWebsite"`
	ContactDiscord   string `json:"contactDiscord"`
	IsLegacy         bool   `json:"isLegacy"`
	ExtensionListURL string `json:"extensionListUrl"`
	TotalCount       int    `json:"totalCount"`
}

