package service

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/je4/filesystem/v2/pkg/osfsrw"
	"github.com/je4/filesystem/v2/pkg/writefs"
	"github.com/je4/filesystem/v2/pkg/zipfs"
	"github.com/je4/gocfl/v2/gocfl/cmd"
	"github.com/je4/gocfl/v2/pkg/ocfl"
	lm "github.com/je4/utils/v2/pkg/logger"
	"ona/models"
)

func ExtractMetadata(storageRootPath string) (models.Object, error) {
	daLogger, lf := lm.CreateLogger("ocfl-reader",
		"",
		nil,
		"ERROR",
		`%{time:2006-01-02T15:04:05.000} %{shortpkg}::%{longfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
	)
	defer lf.Close()

	fsFactory, err := writefs.NewFactory()
	if err != nil {
		return models.Object{}, errors.Wrap(err, "cannot create filesystem factory")
	}
	if err := fsFactory.Register(zipfs.NewCreateFSFunc(), "\\.zip$", writefs.HighFS); err != nil {
		return models.Object{}, errors.Wrap(err, "cannot register zipfs")
	}
	if err := fsFactory.Register(osfsrw.NewCreateFSFunc(), "", writefs.LowFS); err != nil {
		return models.Object{}, errors.Wrap(err, "cannot register zipfs")
	}
	ocflFS, err := fsFactory.Get(storageRootPath)
	if err != nil {
		return models.Object{}, err
	}
	defer func() {
		if err := writefs.Close(ocflFS); err != nil {
			daLogger.Errorf("cannot close filesystem: %v", err)
		}
	}()

	extensionFactory, err := cmd.InitExtensionFactory(map[string]string{},
		"",
		false,
		nil,
		nil,
		nil,
		nil,
		daLogger)
	if err != nil {
		return models.Object{}, errors.Wrap(err, "cannot instantiate extension factory")
	}

	ctx := ocfl.NewContextValidation(context.TODO())
	storageRoot, err := ocfl.LoadStorageRoot(ctx, ocflFS, extensionFactory, daLogger)
	if err != nil {
		return models.Object{}, err
	}
	metadata, err := storageRoot.ExtractMeta("", "")
	if err != nil {
		fmt.Printf("cannot extract metadata from storage root: %v\n", err)
		return models.Object{}, err
	}

	objectMetadata := &ocfl.ObjectMetadata{}
	for _, mapItem := range metadata.Objects {
		objectMetadata = mapItem
	}

	objectRetrieved, ok := objectMetadata.Extension.(map[string]any)
	if !ok {
		fmt.Printf("cannot extract metadata from storage root: %v\n", err)
		return models.Object{}, err
	}

	objectJson := objectRetrieved["NNNN-metafile"].(map[string]any)

	object := models.Object{}
	object.Address = objectJson["address"].(string)
	object.OrganisationAddress = objectJson["organisation_address"].(string)
	alternativeTitlesRow := objectJson["alternative_titles"].([]any)
	for _, item := range alternativeTitlesRow {
		object.AlternativeTitles = append(object.AlternativeTitles, item.(string))
	}
	object.Collection = objectJson["collection"].(string)
	if objectJson["description"] != nil {
		object.Description = objectJson["description"].(string)
	}
	object.CollectionId = objectJson["collection_id"].(string)
	object.Created = objectJson["created"].(string)
	identifiersRaw := objectJson["identifiers"].([]any)
	for _, item := range identifiersRaw {
		object.Identifiers = append(object.Identifiers, item.(string))
	}
	object.IngestWorkflow = objectJson["ingest_workflow"].(string)
	object.LastChanged = objectJson["last_changed"].(string)
	object.Organisation = objectJson["organisation"].(string)
	object.OrganisationId = objectJson["organisation_id"].(string)
	referencesRaw := objectJson["references"].([]any)
	for _, item := range referencesRaw {
		object.References = append(object.References, item.(string))
	}
	setsRaw := objectJson["sets"].([]any)
	for _, item := range setsRaw {
		object.Sets = append(object.Sets, item.(string))
	}
	object.Signature = objectJson["signature"].(string)
	object.Title = objectJson["title"].(string)
	object.User = objectJson["user"].(string)

	return object, nil
}
