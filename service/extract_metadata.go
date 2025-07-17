package service

import (
	"context"
	"emperror.dev/errors"
	"github.com/je4/filesystem/v3/pkg/writefs"
	"github.com/je4/utils/v2/pkg/zLogger"
	pb "github.com/ocfl-archive/dlza-manager/dlzamanagerproto"
	archiveerror "github.com/ocfl-archive/error/pkg/error"
	"github.com/ocfl-archive/gocfl/v2/pkg/ocfl"
	"github.com/ocfl-archive/ona/models"
)

const (
	defaultMimeType = "application/octet-stream"
	defaultPronom   = "UNKNOWN"
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
	storageRoot, err := ocfl.LoadStorageRoot(ctx, ocflFS, g.extensionFactory, g.logger, archiveerror.NewFactory("ona"), "")
	if err != nil {
		return models.Object{}, err
	}
	metadata, err := storageRoot.ExtractMeta("", "")
	if err != nil {
		g.logger.Error().Msgf("cannot extract metadata from storage root: %v\n", err)
		return models.Object{}, err
	}

	return GetObjectFromGocflObject(metadata)
}

func GetObjectFromGocflObject(metadata *ocfl.StorageRootMetadata) (models.Object, error) {
	objectMetadata := &ocfl.ObjectMetadata{}
	for _, mapItem := range metadata.Objects {
		objectMetadata = mapItem
	}
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

func GetFilesFromGocflObject(metadata *ocfl.StorageRootMetadata) []*pb.File {
	object := &ocfl.ObjectMetadata{}
	for _, mapItem := range metadata.Objects {
		object = mapItem
	}
	filesRetrieved := object.Files
	head := object.Head

	files := make([]*pb.File, 0)
	for _, fileRetr := range filesRetrieved {
		file := pb.File{}
		file.Name = fileRetr.VersionName[head]

		if fileRetr.Extension["NNNN-indexer"] != nil {
			extensions := fileRetr.Extension["NNNN-indexer"].(map[string]any)

			file.Pronom = extensions["pronom"].(string)
			if file.Pronom == "" {
				file.Pronom = defaultPronom
			}
			if extensions["size"] != nil {
				file.Size = int64(extensions["size"].(float64))
			}
			if extensions["duration"] != nil {
				file.Duration = int64(extensions["duration"].(float64))
			}
			if extensions["width"] != nil {
				file.Width = int64(extensions["width"].(float64))
			}
			if extensions["height"] != nil {
				file.Height = int64(extensions["height"].(float64))
			}
			file.MimeType = extensions["mimetype"].(string)
			if file.MimeType == "" {
				file.MimeType = defaultMimeType
			}
		} else {
			file.MimeType = defaultMimeType
			file.Pronom = defaultPronom
		}
		files = append(files, &file)
	}
	return files
}
