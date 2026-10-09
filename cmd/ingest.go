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
	ona ingest -q -p C:\Users\123-345.zip -c C:\Users\config.yml -s C:\Users\test.json
	will store 123-345.zip to DLZA without checksum with upload information in test.json. To add checksum you should add a file that contains checksum in the 
	same folder with name 123-345.zip.sha512
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: sendFile,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("json", "j", "", "Path to json file")
	generateCmd.Flags().StringP("path", "p", "", "Path to file")
	generateCmd.Flags().BoolP("quiet", "q", false, "The process information should not be showed")
	generateCmd.Flags().BoolP("background", "b", false, "Do not wait until the order is finished")
	generateCmd.Flags().BoolP("force", "f", false, "Force to archive and retrieve checksum during the process")
	generateCmd.Flags().StringP("status", "s", "", "Path to upload information json file")
}

func sendFile(cmd *cobra.Command, args []string) (err error) {
	statusFilePathRaw, _ := cmd.Flags().GetString("status")
	var statusFilePathCleaned string
	if statusFilePathRaw != "" {
		statusFilePathCleaned = filepath.ToSlash(filepath.Clean(statusFilePathRaw))
	}

	var archivedStatus models.ArchivingStatus
	var obj models.Object
	var fileName string
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]`)

	defer func() {
		if statusFilePathCleaned != "" {
			dateStr := time.Now().Format(time.RFC3339)
			if writeErr := writeStatusFile(statusFilePathCleaned, archivedStatus, dateStr, obj.Signature, fileName, err); writeErr != nil {
				fmt.Printf("could not write status to file: %v\n", writeErr)
			}
		}
	}()
	background, err := cmd.Flags().GetBool("background")
	if err != nil {
		fmt.Println(err)
		return err
	}
	cfgFilePath, err := cmd.Flags().GetString("config")
	if err != nil {
		fmt.Println(err)
		return err
	}

	configObj := service.GetConfig(cfgFilePath)
	ctx := context.Background()
	out := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339Nano}
	zlogger := zerolog.New(out).
		With().
		Timestamp().
		Logger().
		Level(zerolog.ErrorLevel)
	//Level(zerolog.DebugLevel)
	var _zlogger zLogger.ZLogger = &zlogger
	logger := ocfl.NewOCFLLogger(ctx, &zlogger, nil, version.Version1_1, nil)

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		logger.Error().Msg(err.Error())
		return err
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		logger.Error().Msg(err.Error())
		return err
	}

	filePathRaw, _ := cmd.Flags().GetString("path")
	if filePathRaw == "" {
		err = errors.New("You should should specify path")
		logger.Error().Msg(err.Error())
		return err
	}
	filePathCleaned := filepath.ToSlash(filepath.Clean(filePathRaw))

	file, err := os.Open(filePathCleaned)
	if err != nil {
		logger.Error().Msg("could not open file: " + filePathRaw)
		return err
	}
	defer file.Close()

	fileInfo, err := os.Stat(filePathCleaned)
	if err != nil {
		logger.Error().Msgf("cannot read file: %v", err)
		return err
	}
	objectSize := fileInfo.Size()

	jsonPathRow, err := cmd.Flags().GetString("json")
	if err != nil {
		logger.Error().Msg(err.Error())
		return err
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
			logger.Error().Msg(err.Error())
			return err
		}
		if err := csWriter.Close(); err != nil {
			logger.Error().Msgf("cannot close checksum writer %s", err)
			return err
		}
		checksums, err := csWriter.GetChecksums()
		if err != nil {
			logger.Error().Msgf("cannot get checksum %s", err)
		}
		checksum = checksums[checksumType]
	} else {
		fileChecksum, readErr := os.ReadFile(filePathCleaned + "." + checksumType)
		if readErr == nil {
			checksum = strings.Split(string(fileChecksum), separator)[0]
		} else {
			err = errors.New("You should have a checksum file in the folder or use -f flag to produce the checksum ")
			logger.Error().Msg(err.Error())
			return err
		}
	}

	objectJson := ""
	jsonPathCleaned := ""
	sendTwoFiles := false
	obj = models.Object{}
	var objectOcfl inventory.Metadata
	if jsonPathRow != "" {
		jsonPathCleaned = filepath.ToSlash(filepath.Clean(jsonPathRow))
		jsonObject, err := os.ReadFile(jsonPathCleaned)
		if err != nil {
			logger.Error().Msg("could not open json file: " + jsonPathCleaned)
			return err
		}
		err = json.Unmarshal(jsonObject, &objectOcfl)
		if err != nil {
			logger.Error().Msg(err.Error())
			return err
		}

		if objectOcfl.ID != "" {
			obj, err = service.GetObjectFromGocflObjectT(&objectOcfl)
			if err != nil {
				logger.Error().Msg(err.Error())
				return err
			}
			sendTwoFiles = true
		} else {
			err = json.Unmarshal(jsonObject, &obj)
			if err != nil {
				logger.Error().Msg(err.Error())
				return err
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
				return err
			}
		} else {
			// Prepare access to the OCFL directory
			var err error
			destFS, err = writefs.Sub(vfs, ocflPath)
			if err != nil {
				logger.Error().Err(err).Msgf("cannot get filesystem for '%s'", ocflPath)
				return err
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
			return err
		} else if err == nil {

			fis, err := fs.ReadDir(destFS, ".")
			if err != nil {
				logger.Error().Err(err).Msgf("cannot read directory for '%s'", ocflPath)
				return err
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
				return err
			}

			objFsys, err = writefs.Sub(destFS, objF)
			if err != nil {
				logger.Error().Err(err).Msgf("cannot open filesystem for '%s'", filePathCleaned)
				return err
			}
		}

		objLoaded, err := ocfl.LoadObject(ctx, objFsys, nil, logger)
		if err != nil {
			logger.Error().Msgf("failed to load object '%s' at '%s': %v", filePathCleaned, ocflPath, err)
			return err
		}
		defer objLoaded.Close()

		extractor := objLoaded.GetExtractor()
		defer func() { _ = extractor.Close() }()
		metadata, err := extractor.GetMetadata()
		if err != nil {
			logger.Error().Msgf("failed to get metadata for object '%s': %v", filePathCleaned, err)
			return err
		}
		obj, err = service.GetObjectFromGocflObjectT(metadata)
		if err != nil {
			logger.Error().Msgf("failed to convert metadata to object for '%s': %v", filePathCleaned, err)
			return err
		}
		obj.Binary = false
	}
	obj.Checksum = checksum
	obj.Size = objectSize
	fileName = getFileName(filePathCleaned, obj.Signature, re)
	var uploads []*os.File
	if sendTwoFiles && jsonPathCleaned != "" {
		jsonFile, err := os.Open(jsonPathCleaned)
		if err != nil {
			logger.Error().Msg("could not open file: " + jsonPathCleaned)
			return err
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
		return err
	}

	head := "v1"
	if objectPb.Id != "" {
		objectInstancePb, err := service.CheckRawObjectInstanceByObjectId(objectPb.Id, *configObj)
		if err != nil {
			logger.Error().Msgf("could not GetObjectBySignature %s", err)
			return err
		}
		if objectInstancePb.Id == "" {
			objects, err := service.GetObjectsByChecksum(checksum, *configObj)
			if err != nil {
				logger.Error().Msgf("could not get objects from database to check whether object with checksum %s exists", checksum)
				return err
			}
			if len(objects.Objects) != 0 {
				err = errors.Errorf("The file with checksum: %s you are trying to archive already exists in archive\n", checksum)
				logger.Error().Msg(err.Error())
				return err
			}
			head = "v+"
		}
		obj.Id = objectPb.Id
	}
	//checking whether needed amount of locations is available, if yes, delivering partitionId of first location to copy in
	partitionId, err := service.GetStorageLocationsStatusForCollectionAlias(obj.CollectionId, objectSize, obj.Signature, head, *configObj)
	if err != nil {
		logger.Error().Msgf("could not get GetStorageLocationsStatusForCollectionAlias %s", err)
		return err
	}
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	if !r.MatchString(partitionId) {
		err = errors.Errorf("could not get StoragePartition for collection with alias %s", obj.Collection)
		logger.Error().Msg(err.Error())
		return err
	}

	archivedStatus, err = service.CreateStatus(models.ArchivingStatus{Status: initialCopying}, *configObj)
	if err != nil {
		logger.Error().Msg("could not create initial status")
		return err
	}
	ObjectJsonRaw, err := json.Marshal(obj)
	if err != nil {
		logger.Error().Msg(err.Error())
		return err
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
			logger.Error().Msg("could not create client for: " + configObj.Url)
			return err
		}

		// create an upload from a file.
		upload, err := tus.NewUploadFromFile(tusUpload)
		if err != nil {
			logger.Error().Msg("could not upload file: " + path)
			return err
		}
		// create the uploader.
		uploader, err := client.CreateUpload(upload)
		if err != nil {
			logger.Error().Msg("could not create upload for file: " + path + ", with err: " + err.Error())
			return err
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
				logger.Error().Msg(err.Error())
				return err
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
				logger.Error().Msg("could not get initial status with Id: " + archivedStatus.Id)
				return err
			}
			if archivedStatusW.Status != archived && archivedStatusW.Status != errorStatus {
				time.Sleep(10 * time.Second)
			} else {
				fmt.Printf("Status of upload: %s", archivedStatusW.Status)
				archivedStatus = archivedStatusW
				if archivedStatusW.Status == errorStatus {
					return errors.New("upload status is error")
				}
				break
			}
		}

	}
	return nil
}

func getFileName(path string, signature string, re *regexp.Regexp) string {
	extension := filepath.Ext(path)
	fileName := re.ReplaceAllString(signature+extension, "_")
	return fileName
}

func writeStatusFile(statusFilePath string, archivedStatus models.ArchivingStatus, date, signature, fileName string, procErr error) error {
	if statusFilePath == "" {
		return nil
	}
	dir := filepath.Dir(statusFilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	var data []byte
	var marshalErr error

	if procErr != nil {
		output := struct {
			Id    string `json:"id"`
			Error string `json:"error"`
		}{
			Id:    archivedStatus.Id,
			Error: procErr.Error(),
		}
		data, marshalErr = json.MarshalIndent(output, "", "  ")
	} else {
		output := struct {
			models.ArchivingStatus
			Date      string `json:"date"`
			Signature string `json:"signature"`
			FileName  string `json:"fileName"`
		}{
			ArchivingStatus: archivedStatus,
			Date:            date,
			Signature:       signature,
			FileName:        fileName,
		}
		data, marshalErr = json.MarshalIndent(output, "", "  ")
	}

	if marshalErr != nil {
		return marshalErr
	}

	return os.WriteFile(statusFilePath, data, 0644)
}
