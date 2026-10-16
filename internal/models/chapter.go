package models

// ChapterDataClass is the JSON response object for a single chapter.
// Mirrors suwayomi.tachidesk.manga.model.dataclass.ChapterDataClass exactly.
type ChapterDataClass struct {
	ID           int               `json:"id"`
	URL          string            `json:"url"`
	Name         string            `json:"name"`
	UploadDate   int64             `json:"uploadDate"`
	ChapterNumber float32          `json:"chapterNumber"`
	Scanlator    *string           `json:"scanlator"`
	MangaID      int               `json:"mangaId"`
	Read         bool              `json:"read"`
	Bookmarked   bool              `json:"bookmarked"`
	LastPageRead int               `json:"lastPageRead"`
	LastReadAt   int64             `json:"lastReadAt"`
	// Index is the chapter's source order index (starts at 1). Java calls it "index" in JSON.
	Index        int               `json:"index"`
	FetchedAt    int64             `json:"fetchedAt"`
	RealURL      *string           `json:"realUrl"`
	Downloaded   bool              `json:"downloaded"`
	PageCount    int               `json:"pageCount"`
	LastModifiedAt int64           `json:"lastModifiedAt"`
	Version      int64             `json:"version"`
	// Meta is deprecated in V1 API but still returned for frontend compatibility.
	Meta         map[string]string `json:"meta"`
	// ChapterCount is deprecated in V1 API (total chapters of parent manga).
	ChapterCount int               `json:"chapterCount"`
}

// MangaChapterDataClass groups a manga and one of its chapters (used in recent-chapters response).
type MangaChapterDataClass struct {
	Manga   MangaDataClass   `json:"manga"`
	Chapter ChapterDataClass `json:"chapter"`
}

// PaginatedList is a generic paginated response wrapper.
type PaginatedList[T any] struct {
	Page        []T  `json:"page"`
	HasNextPage bool `json:"hasNextPage"`
}

// ChapterBatchEditChange holds the fields that can be changed in a batch edit.
type ChapterBatchEditChange struct {
	IsRead       *bool `json:"isRead"`
	IsBookmarked *bool `json:"isBookmarked"`
	LastPageRead *int  `json:"lastPageRead"`
	Delete       *bool `json:"delete"`
}

// MangaChapterBatchEditInput is the body for POST /manga/{mangaId}/chapter/batch.
type MangaChapterBatchEditInput struct {
	ChapterIDs []int                  `json:"chapterIds"`
	ChangeType ChapterBatchEditChange `json:"change"`
}

// ChapterBatchEditInput is the body for POST /chapter/batch (cross-manga).
type ChapterBatchEditInput struct {
	ChapterIDs []int                  `json:"chapterIds"`
	Change     ChapterBatchEditChange `json:"change"`
}
