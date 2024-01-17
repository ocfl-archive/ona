package models

type File struct {
	Checksum string   `json:"checksum"`
	Name     []string `json:"name"`
	Size     int      `json:"size"`
	MimeType string   `json:"mime_type"`
	Pronom   string   `json:"pronom"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Duration int      `json:"duration"`
	Id       string   `json:"id"`
	ObjectId string   `json:"object_id"`
}
