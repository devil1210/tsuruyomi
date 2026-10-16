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
