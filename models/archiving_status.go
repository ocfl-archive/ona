package models

type ArchivingStatus struct {
	Id          string `json:"id"`
	Status      string `json:"status"`
	LastChanged string `json:"lastChanged"`
}
