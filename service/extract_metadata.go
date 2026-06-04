package service

import (
	"emperror.dev/errors"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl/inventory"
	"github.com/ocfl-archive/ona/models"
)

func GetObjectFromGocflObjectT(objectMetadata *inventory.Metadata) (models.Object, error) {

	objectRetrieved, ok := objectMetadata.Extension.(map[string]any)
	if !ok {
		return models.Object{}, errors.New("cannot extract metadata from storage root")
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
