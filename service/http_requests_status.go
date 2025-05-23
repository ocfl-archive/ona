package service

import (
	"bytes"
	"crypto/tls"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"github.com/ocfl-archive/dlza-manager/dlzamanagerproto"
	pb "github.com/ocfl-archive/dlza-manager/dlzamanagerproto"
	"github.com/ocfl-archive/ona/configuration"
	"github.com/ocfl-archive/ona/models"
	"io"
	"net/http"
)

const (
	aliasAndSize       = "/storage-location/collection/"
	status             = "/status/"
	storageInfo        = "/object-instance/"
	objectInstanceInfo = "/object-instance/signature-and-location/"
	object             = "/object/"
	objectSignature    = "/object/signature/"
	ResultingQuality   = "resulting-quality/"
	NeededQuality      = "needed-quality/"
)

func GetObjectInstancesBySignatureAndLocationsPathName(signature string, config configuration.Config) (*pb.ObjectInstance, error) {
	objectInstance := &pb.ObjectInstance{}
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+objectInstanceInfo+signature+"/"+config.Storage.Name, nil)
	if err != nil {
		return objectInstance, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return objectInstance, err
	}
	err = json.Unmarshal(body, &objectInstance)
	if err != nil {
		return objectInstance, err
	}
	return objectInstance, nil
}
func GetObjectBySignature(signature string, config configuration.Config) (*pb.Object, error) {
	object := &pb.Object{}
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+objectSignature+signature, nil)
	if err != nil {
		return object, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return object, err
	}
	err = json.Unmarshal(body, &object)
	if err != nil {
		return object, err
	}
	return object, nil
}

func GetStorageLocationsStatusForCollectionAlias(alias string, size int64, signature string, head string, config configuration.Config) (string, error) {
	var status pb.Id
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s%s/%d/%s/%s", config.StatusUrl, aliasAndSize, alias, size, signature, head), nil)
	if err != nil {
		return "error", err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return "error", err
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		return "error", err
	}
	return status.Id, nil
}

func GetQualityForObject(id string, resultingOrNeeded string, config configuration.Config) (*pb.SizeAndId, error) {
	quality := &pb.SizeAndId{}
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+object+resultingOrNeeded+id, nil)
	if err != nil {
		return quality, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return quality, err
	}
	err = json.Unmarshal(body, &quality)
	if err != nil {
		return quality, err
	}
	return quality, nil
}

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

func GetObjectsByChecksum(checksum string, config configuration.Config) (*dlzamanagerproto.Objects, error) {
	objects := &dlzamanagerproto.Objects{}
	req, err := http.NewRequest(http.MethodGet, config.StatusUrl+object+checksum, nil)
	if err != nil {
		return objects, err
	}
	body, err := sendRequest(req, config)
	if err != nil {
		return objects, err
	}
	err = json.Unmarshal(body, &objects)
	if err != nil {
		return objects, err
	}
	return objects, nil
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
