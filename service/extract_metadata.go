package service

import (
	"context"
	"github.com/je4/filesystem/v3/pkg/writefs"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/ocfl-archive/gocfl/v2/pkg/ocfl"
	"github.com/ocfl-archive/ona/models"
)

func NewGocfl(extensionFactory *ocfl.ExtensionFactory, factory *writefs.Factory, logger zLogger.ZLogger) *Gocfl {
	return &Gocfl{extensionFactory: extensionFactory, fsFactory: factory, logger: logger}
}

type Gocfl struct {
	extensionFactory *ocfl.ExtensionFactory
	fsFactory        *writefs.Factory
	logger           zLogger.ZLogger
}

func (g *Gocfl) ExtractMetadata(storageRootPath string) (models.Object, error) {

	ocflFS, err := g.fsFactory.Get(storageRootPath, true)
	if err != nil {
		return models.Object{}, err
	}
	defer func() {
		if err := writefs.Close(ocflFS); err != nil {
			g.logger.Error().Msgf("cannot close filesystem: %v", err)
		}
	}()

	ctx := ocfl.NewContextValidation(context.TODO())
	storageRoot, err := ocfl.LoadStorageRoot(ctx, ocflFS, g.extensionFactory, g.logger)
	if err != nil {
		return models.Object{}, err
	}
	metadata, err := storageRoot.ExtractMeta("", "")
	if err != nil {
		g.logger.Error().Msgf("cannot extract metadata from storage root: %v\n", err)
		return models.Object{}, err
	}

	objectMetadata := &ocfl.ObjectMetadata{}
	for _, mapItem := range metadata.Objects {
		objectMetadata = mapItem
	}

	objectRetrieved, ok := objectMetadata.Extension.(map[string]any)
	if !ok {
		g.logger.Error().Msgf("cannot extract metadata from storage root: %v\n", err)
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
	if objectJson["holding"] != nil {
		object.Holding = objectJson["holding"].(string)
	}
	if objectJson["expiration"] != nil {
		object.Expiration = objectJson["expiration"].(string)
	}
	object.OrganisationId = objectJson["organisation_id"].(string)
	referencesRaw := objectJson["references"].([]any)
	for _, item := range referencesRaw {
		object.References = append(object.References, item.(string))
	}
	setsRaw := objectJson["sets"].([]any)
	for _, item := range setsRaw {
		object.Sets = append(object.Sets, item.(string))
	}
	if objectJson["authors"] != nil {
		authorsRaw := objectJson["authors"].([]any)
		for _, item := range authorsRaw {
			object.Authors = append(object.Authors, item.(string))
		}
	}
	if objectJson["keywords"] != nil {
		keywordsRaw := objectJson["keywords"].([]any)
		for _, item := range keywordsRaw {
			object.Keywords = append(object.Keywords, item.(string))
		}
	}
	object.Signature = objectJson["signature"].(string)
	object.Title = objectJson["title"].(string)
	object.User = objectJson["user"].(string)

	return object, nil
}
