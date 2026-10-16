package models

type DownloadState string

const (
	DownloadStateQueued      DownloadState = "Queued"
	DownloadStateDownloading DownloadState = "Downloading"
	DownloadStateFinished    DownloadState = "Finished"
	DownloadStateError       DownloadState = "Error"
)

type DownloadQueueItem struct {
	ChapterID    int           `json:"chapterId"`
	ChapterIndex int           `json:"chapterIndex"`
	MangaID      int           `json:"mangaId"`
	SourceID     int64         `json:"sourceId"`
	Position     int           `json:"position"`
	PageCount    int           `json:"pageCount"`
	State        DownloadState `json:"state"`
	Progress     float32       `json:"progress"`
	Tries        int           `json:"tries"`
}

type DownloadChapter struct {
	ChapterIndex int              `json:"chapterIndex"`
	MangaID      int              `json:"mangaId"`
	Chapter      ChapterDataClass `json:"chapter"`
	Manga        MangaDataClass   `json:"manga"`
	Position     int              `json:"position"`
	State        DownloadState    `json:"state"`
	Progress     float32          `json:"progress"`
	Tries        int              `json:"tries"`
}

type DownloadStatus struct {
	Status string               `json:"status"` // "Started" | "Stopped"
	Queue  []DownloadQueueItem  `json:"queue"`
}

type OldDownloadStatus struct {
	Status string            `json:"status"`
	Queue  []DownloadChapter `json:"queue"`
}

type DownloadUpdate struct {
	ChapterID         int           `json:"chapterId"`
	MangaID           *int          `json:"mangaId,omitempty"`
	State             DownloadState `json:"state"`
	Progress          float32       `json:"progress"`
	DownloadQueueItem *DownloadQueueItem `json:"downloadQueueItem,omitempty"`
}

type EnqueueInput struct {
	ChapterIds []int `json:"chapterIds"`
}
