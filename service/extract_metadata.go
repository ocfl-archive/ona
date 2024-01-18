package service

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/je4/filesystem/v2/pkg/osfsrw"
	"github.com/je4/filesystem/v2/pkg/writefs"
	"github.com/je4/filesystem/v2/pkg/zipfs"
	"github.com/je4/gocfl/v2/pkg/extension"
	"github.com/je4/gocfl/v2/pkg/ocfl"
	"github.com/je4/gocfl/v2/pkg/subsystem/migration"
	"github.com/je4/gocfl/v2/pkg/subsystem/thumbnail"
	ironmaiden "github.com/je4/indexer/v2/pkg/indexer"
	lm "github.com/je4/utils/v2/pkg/logger"
	"github.com/op/go-logging"
	"io/fs"
	"ona/models"
)

func initExtensionFactory(extensionParams map[string]string, indexerAddr string, indexerLocalCache bool, indexerActions *ironmaiden.ActionDispatcher, migration *migration.Migration, thumbnail *thumbnail.Thumbnail, sourceFS fs.FS, logger *logging.Logger) (*ocfl.ExtensionFactory, error) {
	extensionFactory, err := ocfl.NewExtensionFactory(extensionParams, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot instantiate extension factory")
	}

	extensionFactory.AddCreator(extension.DigestAlgorithmsName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewDigestAlgorithmsFS(fsys)
	})

	extensionFactory.AddCreator(extension.StorageLayoutFlatDirectName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewStorageLayoutFlatDirectFS(fsys)
	})

	extensionFactory.AddCreator(extension.StorageLayoutHashAndIdNTupleName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewStorageLayoutHashAndIdNTupleFS(fsys)
	})

	extensionFactory.AddCreator(extension.StorageLayoutHashedNTupleName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewStorageLayoutHashedNTupleFS(fsys)
	})

	extensionFactory.AddCreator(extension.FlatOmitPrefixStorageLayoutName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewFlatOmitPrefixStorageLayoutFS(fsys)
	})

	extensionFactory.AddCreator(extension.NTupleOmitPrefixStorageLayoutName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewNTupleOmitPrefixStorageLayoutFS(fsys)
	})

	extensionFactory.AddCreator(extension.DirectCleanName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewDirectCleanFS(fsys)
	})

	extensionFactory.AddCreator(extension.PathDirectName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewPathDirectFS(fsys)
	})

	extensionFactory.AddCreator(extension.StorageLayoutPairTreeName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewStorageLayoutPairTreeFS(fsys)
	})

	extensionFactory.AddCreator(ocfl.ExtensionManagerName, func(fsys fs.FS) (ocfl.Extension, error) {
		return ocfl.NewInitialDummyFS(fsys)
	})

	extensionFactory.AddCreator(extension.ContentSubPathName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewContentSubPathFS(fsys)
	})

	extensionFactory.AddCreator(extension.MetaFileName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewMetaFileFS(fsys)
	})

	extensionFactory.AddCreator(extension.IndexerName, func(fsys fs.FS) (ocfl.Extension, error) {
		ext, err := extension.NewIndexerFS(fsys, indexerAddr, indexerActions, indexerLocalCache, logger)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create new indexer from filesystem")
		}
		return ext, nil
	})

	extensionFactory.AddCreator(extension.MigrationName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewMigrationFS(fsys, migration, logger)
	})

	extensionFactory.AddCreator(extension.ThumbnailName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewThumbnailFS(fsys, thumbnail, logger)
	})

	extensionFactory.AddCreator(extension.FilesystemName, func(fsys fs.FS) (ocfl.Extension, error) {
		return extension.NewFilesystemFS(fsys, logger)
	})

	return extensionFactory, nil
}

func ExtractMetadata(storageRootPath string) ([]models.File, error) {
	daLogger, lf := lm.CreateLogger("ocfl-reader",
		"",
		nil,
		"ERROR",
		`%{time:2006-01-02T15:04:05.000} %{shortpkg}::%{longfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
	)
	defer lf.Close()

	fsFactory, err := writefs.NewFactory()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create filesystem factory")
	}
	if err := fsFactory.Register(zipfs.NewCreateFSFunc(), "\\.zip$", writefs.HighFS); err != nil {
		return nil, errors.Wrap(err, "cannot register zipfs")
	}
	if err := fsFactory.Register(osfsrw.NewCreateFSFunc(), "", writefs.LowFS); err != nil {
		return nil, errors.Wrap(err, "cannot register zipfs")
	}
	ocflFS, err := fsFactory.Get(storageRootPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := writefs.Close(ocflFS); err != nil {
			daLogger.Errorf("cannot close filesystem: %v", err)
		}
	}()

	extensionFactory, err := initExtensionFactory(map[string]string{},
		"",
		false,
		nil,
		nil,
		nil,
		nil,
		daLogger)
	if err != nil {
		return nil, errors.Wrap(err, "cannot instantiate extension factory")
	}

	ctx := ocfl.NewContextValidation(context.TODO())
	storageRoot, err := ocfl.LoadStorageRoot(ctx, ocflFS, extensionFactory, daLogger)
	if err != nil {
		return nil, err
	}
	metadata, err := storageRoot.ExtractMeta("", "")
	if err != nil {
		fmt.Printf("cannot extract metadata from storage root: %v\n", err)
		return nil, err
	}

	object := &ocfl.ObjectMetadata{}
	for _, mapItem := range metadata.Objects {
		object = mapItem
	}
	filesRetrieved := object.Files
	head := object.Head

	files := make([]models.File, 0)
	for index, fileRetr := range filesRetrieved {
		file := models.File{}
		file.Checksum = index
		file.Name = fileRetr.VersionName[head]

		extensions := fileRetr.Extension["NNNN-indexer"]
		if extensions != nil {
			switch v := extensions.(type) {
			case *ironmaiden.ResultV2:
				file.Size = int(v.Size)
				file.Pronom = v.Pronom
				file.Duration = int(v.Duration)
				file.Width = int(v.Width)
				file.Height = int(v.Height)
				file.MimeType = v.Mimetype
			}

		}
		files = append(files, file)
	}
	return files, nil
}
