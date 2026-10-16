package models

// MangaStatus represents the publication status of a manga.
// Values must match the Java enum suwayomi.tachidesk.manga.model.table.MangaStatus
// which maps integer DB values to string names in the JSON response.
type MangaStatus string

const (
	MangaStatusUnknown           MangaStatus = "UNKNOWN"
	MangaStatusOngoing           MangaStatus = "ONGOING"
	MangaStatusCompleted         MangaStatus = "COMPLETED"
	MangaStatusLicensed          MangaStatus = "LICENSED"
	MangaStatusPublishingFinished MangaStatus = "PUBLISHING_FINISHED"
	MangaStatusCancelled         MangaStatus = "CANCELLED"
	MangaStatusOnHiatus          MangaStatus = "ON_HIATUS"
)

// MangaStatusFromInt converts the integer stored in PostgreSQL to the string name
// the JSON API exposes. Matches Java MangaStatus.valueOf(Int).
func MangaStatusFromInt(v int) MangaStatus {
	switch v {
	case 1:
		return MangaStatusOngoing
	case 2:
		return MangaStatusCompleted
	case 3:
		return MangaStatusLicensed
	case 4:
		return MangaStatusPublishingFinished
	case 5:
		return MangaStatusCancelled
	case 6:
		return MangaStatusOnHiatus
	default:
		return MangaStatusUnknown
	}
}

// UpdateStrategy mirrors eu.kanade.tachiyomi.source.model.UpdateStrategy.
type UpdateStrategy string

const (
	UpdateStrategyAlwaysUpdate UpdateStrategy = "ALWAYS_UPDATE"
	UpdateStrategyOnlyFetchOnce UpdateStrategy = "ONLY_FETCH_ONCE"
)

// MangaDataClass is the primary JSON response object for manga.
// Field names and types must match ChapterDataClass exactly as produced by the Java server.
type MangaDataClass struct {
	ID                    int            `json:"id"`
	SourceID              string         `json:"sourceId"`
	URL                   string         `json:"url"`
	Title                 string         `json:"title"`
	ThumbnailURL          *string        `json:"thumbnailUrl"`
	ThumbnailURLLastFetched int64         `json:"thumbnailUrlLastFetched"`
	Initialized           bool           `json:"initialized"`
	Artist                *string        `json:"artist"`
	Author                *string        `json:"author"`
	Description           *string        `json:"description"`
	Genre                 []string       `json:"genre"`
	Status                MangaStatus    `json:"status"`
	InLibrary             bool           `json:"inLibrary"`
	InLibraryAt           int64          `json:"inLibraryAt"`
	Source                *SourceDataClass `json:"source"`
	RealURL               *string        `json:"realUrl"`
	LastFetchedAt         *int64         `json:"lastFetchedAt"`
	ChaptersLastFetchedAt *int64         `json:"chaptersLastFetchedAt"`
	UpdateStrategy        UpdateStrategy `json:"updateStrategy"`
	FreshData             bool           `json:"freshData"`
	UnreadCount           *int64         `json:"unreadCount"`
	DownloadCount         *int64         `json:"downloadCount"`
	ChapterCount          *int64         `json:"chapterCount"`
	LastReadAt            *int64         `json:"lastReadAt"`
	LastChapterRead       *ChapterDataClass `json:"lastChapterRead"`
	Age                   *int64         `json:"age"`
	ChaptersAge           *int64         `json:"chaptersAge"`
	Trackers              []MangaTrackerDataClass `json:"trackers"`
	LastModifiedAt        int64          `json:"lastModifiedAt"`
	Version               int64          `json:"version"`
	Meta                  map[string]string `json:"meta"`
}

// PagedMangaListDataClass is the paginated manga list response.
type PagedMangaListDataClass struct {
	MangaList   []MangaDataClass `json:"mangaList"`
	HasNextPage bool             `json:"hasNextPage"`
}

// MangaTrackerDataClass holds tracking info attached to a manga.
type MangaTrackerDataClass struct {
	Record  *TrackRecordDataClass `json:"record"`
	Tracker TrackerDataClass      `json:"tracker"`
}
