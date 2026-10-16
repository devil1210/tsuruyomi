package models

type TrackRecordDataClass struct {
	ID        int     `json:"id"`
	MangaID   int     `json:"mangaId"`
	TrackerID int     `json:"trackerId"`
	RemoteID  string  `json:"remoteId"`
	Title     string  `json:"title"`
	LastPages int     `json:"lastPages"`
	Status    int     `json:"status"`
	Score     float32 `json:"score"`
}

type TrackerDataClass struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
