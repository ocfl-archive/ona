package service

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"io"
	"net/http"
	"ona/models"
	"time"
)

func GetStatus(id string) (models.ArchivingStatus, error) {
	config := GetConfig()
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+id, nil)
	if err != nil {
		return models.ArchivingStatus{}, err
	}

	return sendRequest(req)
}

func CreateStatus(status models.ArchivingStatus) (models.ArchivingStatus, error) {
	archivingStatus := models.ArchivingStatus{}
	buf := bytes.Buffer{}
	err := json.NewEncoder(&buf).Encode(status)
	if err != nil {
		return archivingStatus, err
	}
	config := GetConfig()
	req, err := http.NewRequest(http.MethodPost, config.StatusUrl, &buf)
	if err != nil {
		return archivingStatus, err
	}
	return sendRequest(req)
}

func sendRequest(req *http.Request) (models.ArchivingStatus, error) {
	archivingStatus := models.ArchivingStatus{}
	bearer, err := GetBearer()
	req.Header.Add("Authorization", bearer)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		if resp.StatusCode != 200 {
			return archivingStatus, errors.New("Status has status code: " + string(resp.StatusCode))
		}
	} else {
		return archivingStatus, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return archivingStatus, err
	}
	err = json.Unmarshal(body, &archivingStatus)
	if err != nil {
		return archivingStatus, err
	}
	return archivingStatus, nil
}
