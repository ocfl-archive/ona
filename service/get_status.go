package service

import (
	"emperror.dev/errors"
	"encoding/json"
	"io"
	"net/http"
	"ona/models"
	"time"
)

func GetStatus(id string) (models.ArchivingStatus, error) {
	archivingStatus := models.ArchivingStatus{}
	config := GetConfig()
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+id, nil)
	if err != nil {
		return archivingStatus, err
	}

	bearer, err := GetBearer()
	req.Header.Add("Authorization", bearer)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		if resp.StatusCode != 200 {
			return archivingStatus, errors.New("Status was not found or error appeared")
		}
	} else {
		return archivingStatus, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return archivingStatus, err
	}
	_ = json.Unmarshal(body, &archivingStatus)

	return archivingStatus, nil
}
