package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/eventials/go-tus"
	checksumImp "github.com/je4/utils/v2/pkg/checksum"
	"github.com/je4/utils/v2/pkg/zLogger"
	pb "github.com/ocfl-archive/dlza-manager/dlzamanagerproto"
	"github.com/ocfl-archive/filesystem/pkg/vfsrw"
	"github.com/ocfl-archive/filesystem/pkg/writefs"
	"github.com/ocfl-archive/filesystem/pkg/zipfs"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl/inventory"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl/ocflerrors"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl/util"
	"github.com/ocfl-archive/gocfl/v3/pkg/ocfl/version"
	"github.com/ocfl-archive/ona/models"
	"github.com/ocfl-archive/ona/service"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

const (
	initialCopying = "initial copying"
	archived       = "archived"
	errorStatus    = "error"
	checksumType   = "sha512"
	separator      = " *"
)

var generateCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Send files to storage",
	Long: `Send files to storage. Only a link to zip file should be provided.
	To fill checksum field in data base you should have a file with checksum in the same folder as the file to be stored
	and named the same way with addition *.sha512
	For example:
	ona ingest -q -p C:\Users\123-345.zip -c C:\Users\config.yml
	will store 123-345.zip to DLZA without checksum. To add checksum you should add a file that contains checksum in the 
	same folder with name 123-345.zip.sha512
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: sendFile,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("json", "j", "", "Path to json file")
	generateCmd.Flags().StringP("path", "p", "", "Path to file")
	generateCmd.Flags().BoolP("quiet", "q", false, "The process information should not be showed")
	generateCmd.Flags().BoolP("background", "b", false, "Do not wait until the order is finished")
	generateCmd.Flags().BoolP("force", "f", false, "Force to archive and retrieve checksum during the process")
}

func sendFile(cmd *cobra.Command, args []string) {
	background, err := cmd.Flags().GetBool("background")
	if err != nil {
		fmt.Println(err)
		return
	}
	cfgFilePath, err := cmd.Flags().GetString("config")
	if err != nil {
		fmt.Println(err)
		return
	}

	configObj := service.GetConfig(cfgFilePath)
	ctx := context.Background()
	out := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339Nano}
	zlogger := zerolog.New(out).
		With().
		Timestamp().
		Logger().
		Level(zerolog.ErrorLevel)
	var _zlogger zLogger.ZLogger = &zlogger
	logger := ocfl.NewOCFLLogger(ctx, &zlogger, nil, version.Version1_1, nil)

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		logger.Error().Msgf(err.Error())
		return
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		logger.Error().Msgf(err.Error())
		return
	}

	filePathRaw, _ := cmd.Flags().GetString("path")
	if filePathRaw == "" {
		logger.Error().Msgf("You should should specify path")
		return
	}
	filePathCleaned := filepath.ToSlash(filepath.Clean(filePathRaw))

	file, err := os.Open(filePathCleaned)
	if err != nil {
		logger.Error().Msgf("could not open file: " + filePathRaw)
		return
	}
	defer file.Close()

	fileInfo, err := os.Stat(filePathCleaned)
	if err != nil {
		logger.Error().Msgf("cannot read file: %v", err)
		return
	}
	objectSize := fileInfo.Size()

	jsonPathRow, err := cmd.Flags().GetString("json")
	if err != nil {
		logger.Error().Msgf(err.Error())
		return
	}
	checksum := ""
	if force {
		targetFP := io.Discard
		csWriter, err := checksumImp.NewChecksumWriter(
			[]checksumImp.DigestAlgorithm{checksumType},
			targetFP,
		)
		_, err = io.Copy(csWriter, file)
		if err != nil {
			logger.Error().Msgf(err.Error())
			return
		}
		if err := csWriter.Close(); err != nil {
			logger.Error().Msgf("cannot close checksum writer %s", err)
			return
		}
		checksums, err := csWriter.GetChecksums()
		if err != nil {
			logger.Error().Msgf("cannot get checksum %s", err)
		}
		checksum = checksums[checksumType]
	} else {
		fileChecksum, err := os.ReadFile(filePathCleaned + "." + checksumType)
		if err == nil {
			checksum = strings.Split(string(fileChecksum), separator)[0]
		} else {
			logger.Error().Msgf("You should have a checksum file in the folder or use -f flag to produce the checksum ")
			return
		}
	}

	objectJson := ""
	jsonPathCleaned := ""
	sendTwoFiles := false
	obj := models.Object{}
	var objectOcfl inventory.Metadata
	if jsonPathRow != "" {
		jsonPathCleaned = filepath.ToSlash(filepath.Clean(jsonPathRow))
		jsonObject, err := os.ReadFile(jsonPathCleaned)
		if err != nil {
			logger.Error().Msgf("could not open json file: " + jsonPathCleaned)
			return
		}
		err = json.Unmarshal(jsonObject, &objectOcfl)
		if err != nil {
			logger.Error().Msgf(err.Error())
			return
		}

		if objectOcfl.ID != "" {
			obj, err = service.GetObjectFromGocflObjectT(&objectOcfl)
			if err != nil {
				logger.Error().Msgf(err.Error())
				return
			}
			sendTwoFiles = true
		} else {
			err = json.Unmarshal(jsonObject, &obj)
			if err != nil {
				logger.Error().Msgf(err.Error())
				return
			}
		}
		obj.Binary = true
	} else {

		cfg := vfsrw.Config{}
		vfs, err := vfsrw.NewFS(cfg, _zlogger)
		if err != nil {
			log.Fatalf("failed to create vfs: %v", err)
		}
		defer vfs.Close()

		if err := vfsrw.AddLocal(vfs, nil); err != nil {
			logger.Fatal().Err(err).Msg("failed to add local filesystem")
		}

		ocflPath := writefs.RealPath(vfs, filePathCleaned)
		var destFS fs.FS
		if strings.HasSuffix(strings.ToLower(ocflPath), ".zip") {
			var err error
			destFS, err = zipfs.NewFSFile(vfs, ocflPath, logger.Logger())
			if err != nil {
				logger.Error().Err(err).Msgf("cannot open zip filesystem for '%s'", ocflPath)
				return
			}
		} else {
			// Prepare access to the OCFL directory
			var err error
			destFS, err = writefs.Sub(vfs, ocflPath)
			if err != nil {
				logger.Error().Err(err).Msgf("cannot get filesystem for '%s'", ocflPath)
				return
			}
		}
		defer func() {
			if err := writefs.Close(destFS); err != nil {
				logger.Error().Err(err).Msgf("cannot close filesystem for '%s'", ocflPath)
			}
		}()
		objFsys := destFS
		_, err = util.GetStorageRootVersion(destFS)
		if err != nil && !errors.Is(err, ocflerrors.ErrVersionNone) {
			logger.Error().Err(err).Msgf("cannot get storage root version for '%s'", ocflPath)
			return
		} else if err == nil {

			fis, err := fs.ReadDir(destFS, ".")
			if err != nil {
				logger.Error().Err(err).Msgf("cannot read directory for '%s'", ocflPath)
				return
			}
			var objF string
			for _, fi := range fis {
				if fi.IsDir() && fi.Name() != "extensions" {
					objF = fi.Name()
					break
				}
			}
			if objF == "" {
				logger.Error().Msgf("cannot find OCFL object directory for '%s'", ocflPath)
				return
			}

			objFsys, err = writefs.Sub(destFS, objF)
			if err != nil {
				logger.Error().Err(err).Msgf("cannot open filesystem for '%s'", filePathCleaned)
				return
			}
		}

		objLoaded, err := ocfl.LoadObject(ctx, objFsys, nil, logger)
		if err != nil {
			logger.Error().Msgf("failed to load object '%s' at '%s': %v", filePathCleaned, ocflPath, err)
			return
		}
		defer objLoaded.Close()

		extractor := objLoaded.GetExtractor()
		defer func() { _ = extractor.Close() }()
		metadata, err := extractor.GetMetadata()
		if err != nil {
			logger.Error().Msgf("failed to get metadata for object '%s': %v", filePathCleaned, err)
			return
		}
		obj, err = service.GetObjectFromGocflObjectT(metadata)
		if err != nil {
			logger.Error().Msgf("failed to convert metadata to object for '%s': %v", filePathCleaned, err)
			return
		}
		obj.Binary = false
	}
	obj.Checksum = checksum
	obj.Size = objectSize
	var uploads []*os.File
	if sendTwoFiles && jsonPathCleaned != "" {
		jsonFile, err := os.Open(jsonPathCleaned)
		if err != nil {
			logger.Error().Msgf("could not open file: " + jsonPathCleaned)
			return
		}
		defer jsonFile.Close()
		if jsonFile != nil {
			uploads = append(uploads, jsonFile)
		}
	}
	uploads = append(uploads, file)

	objectPb, err := service.GetObjectBySignature(obj.Signature, *configObj)
	if err != nil {
		logger.Error().Msgf("could not GetObjectBySignature %s", err)
		return
	}

	head := "v1"
	if objectPb.Id != "" {
		objectInstancePb, err := service.CheckRawObjectInstanceByObjectId(objectPb.Id, *configObj)
		if err != nil {
			logger.Error().Msgf("could not GetObjectBySignature %s", err)
			return
		}
		if objectInstancePb.Id == "" {
			objects, err := service.GetObjectsByChecksum(checksum, *configObj)
			if err != nil {
				logger.Error().Msgf("could not get objects from database to check whether object with checksum %s exists", checksum)
				return
			}
			if len(objects.Objects) != 0 {
				logger.Error().Msgf("The file with checksum: %s you are trying to archive already exists in archive\n", checksum)
				return
			}
			head = "v+"
		}
		obj.Id = objectPb.Id
	}
	//checking whether needed amount of locations is available, if yes, delivering partitionId of first location to copy in
	partitionId, err := service.GetStorageLocationsStatusForCollectionAlias(obj.CollectionId, objectSize, obj.Signature, head, *configObj)
	if err != nil {
		logger.Error().Msgf("could not get GetStorageLocationsStatusForCollectionAlias %s", err)
		return
	}
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	if !r.MatchString(partitionId) {
		logger.Error().Msgf("could not get StoragePartition for collection with alias %s", obj.Collection)
		return
	}

	archivedStatus, err := service.CreateStatus(models.ArchivingStatus{Status: initialCopying}, *configObj)
	if err != nil {
		logger.Error().Msgf("could not create initial status")
		return
	}
	ObjectJsonRaw, err := json.Marshal(obj)
	if err != nil {
		logger.Error().Msgf(err.Error())
		return
	}
	objectJson = string(ObjectJsonRaw)
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
	httpClient := &http.Client{Transport: customTransport}
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]`)
	for index, tusUpload := range uploads {
		path := ""
		severalObjects := ""
		if len(uploads) > 1 {
			severalObjects = strconv.Itoa(index)
		}
		if len(uploads) > 1 && index == 0 {
			path = jsonPathCleaned
		} else {
			path = filePathCleaned
		}

		// create the tus client.
		client, err := tus.NewClient(configObj.Url, &tus.Config{ChunkSize: configObj.ChunkSize, Header: map[string][]string{"Authorization": {configObj.Key},
			"ObjectJson": {objectJson}, "Collection": {obj.CollectionId}, "StatusId": {archivedStatus.Id}, "Checksum": {checksum}, "FileName": {getFileName(path, obj.Signature, re)}, "PartitionId": {partitionId}, "SeveralObjects": {severalObjects}}, HttpClient: httpClient})
		if err != nil {
			logger.Error().Msgf("could not create client for: " + configObj.Url)
			return
		}

		// create an upload from a file.
		upload, err := tus.NewUploadFromFile(tusUpload)
		if err != nil {
			logger.Error().Msgf("could not upload file: " + path)
			return
		}
		// create the uploader.
		uploader, err := client.CreateUpload(upload)
		if err != nil {
			logger.Error().Msgf("could not create upload for file: " + path + ", with err: " + err.Error())
			return
		}
		if obj.Id == "" {
			objectWithInfo := &pb.ObjectAndFile{}
			objectPbF := &pb.Object{}
			//statusId field is used to transfer partition id
			objectWithInfo.StatusId = partitionId
			objectWithInfo.FileName = getFileName(filePathCleaned, obj.Signature, re)

			objectPbF.Size = obj.Size
			objectPbF.Signature = obj.Signature
			objectPbF.CollectionId = obj.CollectionId
			objectPbF.Collection = obj.Collection
			objectPbF.Binary = obj.Binary
			objectPbF.Address = obj.Address
			objectPbF.AlternativeTitles = obj.AlternativeTitles
			objectPbF.Checksum = obj.Checksum
			objectPbF.Authors = obj.Authors
			objectPbF.Description = obj.Description
			objectPbF.Keywords = obj.Keywords
			objectPbF.Created = obj.Created
			objectPbF.Expiration = obj.Expiration
			objectPbF.Head = head
			objectPbF.Holding = obj.Holding
			objectPbF.Identifiers = obj.Identifiers
			objectPbF.IngestWorkflow = obj.IngestWorkflow
			objectPbF.LastChanged = obj.LastChanged
			objectPbF.References = obj.References
			objectPbF.Sets = obj.Sets
			objectPbF.Title = obj.Title
			objectPbF.User = obj.User
			objectWithInfo.Object = objectPbF
			obj.Id = "exists"

			err = service.CreateObjectAndInstance(objectWithInfo, *configObj)
			if err != nil {
				logger.Error().Msgf(err.Error())
				return
			}
		}

		if !quiet {
			// start the uploading process.
			go func() {
				uploader.Upload()
			}()
			fmt.Println("Upload...")
			bar := progressbar.NewOptions64(
				upload.Size(),
				progressbar.OptionSetDescription(""),
				progressbar.OptionSetWriter(os.Stdout),
				progressbar.OptionSetWidth(10),
				progressbar.OptionThrottle(65*time.Millisecond),
				progressbar.OptionOnCompletion(func() {
					fmt.Fprint(os.Stdout, "\nUpload is finished. Upload Id: "+archivedStatus.Id+" \n")
				}),
				progressbar.OptionSpinnerType(14),
				progressbar.OptionFullWidth(),
				progressbar.OptionSetRenderBlankState(true),
			)

			size := upload.Size()
			for {
				if upload.Finished() {
					bar.Set(int(size))
					break
				}
				offset := upload.Offset()
				bar.Set(int(offset))
			}
		} else {
			uploader.Upload()
		}
	}

	if !background {
		for {
			archivedStatusW, err := service.GetStatus(archivedStatus.Id, *configObj)
			if err != nil {
				logger.Error().Msgf("could not get initial status with Id: " + archivedStatus.Id)
				return
			}
			if archivedStatusW.Status != archived && archivedStatusW.Status != errorStatus {
				time.Sleep(10 * time.Second)
			} else {
				fmt.Printf("Status of upload: %s", archivedStatusW.Status)
				break
			}
		}

	}
}

func getFileName(path string, signature string, re *regexp.Regexp) string {
	extension := filepath.Ext(path)
	fileName := re.ReplaceAllString(signature+extension, "_")
	return fileName
}
