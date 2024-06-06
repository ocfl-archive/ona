package service

import (
	"bytes"
	"crypto/tls"
	"emperror.dev/errors"
	"encoding/json"
	"gitlab.switch.ch/ub-unibas/dlza/dlza-manager/dlzamanagerproto"
	"io"
	"net/http"
	"ona/configuration"
	"ona/models"
)

const (
	status      = "/status/"
	storageInfo = "/object-instance/"
)

func GetStatus(id string, config configuration.Config) (models.ArchivingStatus, error) {
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+status+id, nil)
	if err != nil {
		return models.ArchivingStatus{}, err
	}
	body, err := sendRequest(req, config)
	archivingStatus := models.ArchivingStatus{}
	if err != nil {
		return archivingStatus, err
	}
	err = json.Unmarshal(body, &archivingStatus)
	if err != nil {
		return archivingStatus, err
	}
	return archivingStatus, nil
}

func GetObjectInstancesByName(name string, config configuration.Config) (*dlzamanagerproto.ObjectInstances, error) {
	objectInstances := &dlzamanagerproto.ObjectInstances{}
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+storageInfo+name, nil)
	if err != nil {
		return objectInstances, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return objectInstances, err
	}
	err = json.Unmarshal(body, &objectInstances)
	if err != nil {
		return objectInstances, err
	}
	return objectInstances, nil
}

func CreateStatus(statusObj models.ArchivingStatus, config configuration.Config) (models.ArchivingStatus, error) {
	archivingStatus := models.ArchivingStatus{}
	buf := bytes.Buffer{}
	err := json.NewEncoder(&buf).Encode(statusObj)
	if err != nil {
		return archivingStatus, err
	}
	req, err := http.NewRequest(http.MethodPost, config.StatusUrl+status, &buf)
	if err != nil {
		return archivingStatus, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return archivingStatus, err
	}
	err = json.Unmarshal(body, &archivingStatus)
	if err != nil {
		return archivingStatus, err
	}
	return archivingStatus, nil
}

func sendRequest(req *http.Request, config configuration.Config) ([]byte, error) {
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

	bearer, err := GetBearer(config)
	req.Header.Add("Authorization", bearer)

	resp, err := client.Do(req)
	if err == nil {
		if resp.StatusCode != 200 {
			return nil, errors.New("Status has status code: " + string(resp.StatusCode))
		}
	} else {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
