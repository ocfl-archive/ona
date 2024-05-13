package service

import (
	"bytes"
	"crypto/tls"
	"emperror.dev/errors"
	"encoding/json"
	"io"
	"net/http"
	"ona/configuration"
	"ona/models"
)

func GetStatus(id string, config configuration.Config) (models.ArchivingStatus, error) {
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+id, nil)
	if err != nil {
		return models.ArchivingStatus{}, err
	}

	return sendRequest(req, config)
}

func CreateStatus(status models.ArchivingStatus, config configuration.Config) (models.ArchivingStatus, error) {
	archivingStatus := models.ArchivingStatus{}
	buf := bytes.Buffer{}
	err := json.NewEncoder(&buf).Encode(status)
	if err != nil {
		return archivingStatus, err
	}
	req, err := http.NewRequest(http.MethodPost, config.StatusUrl, &buf)
	if err != nil {
		return archivingStatus, err
	}
	return sendRequest(req, config)
}

func sendRequest(req *http.Request, config configuration.Config) (models.ArchivingStatus, error) {
	defaultTransport := http.DefaultTransport.(*http.Transport)

	// Create new Transport that ignores self-signed SSL
	customTransport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: customTransport}

	archivingStatus := models.ArchivingStatus{}
	bearer, err := GetBearer(config)
	req.Header.Add("Authorization", bearer)

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
