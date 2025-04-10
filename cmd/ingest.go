package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/je4/filesystem/v3/pkg/osfsrw"
	"github.com/je4/filesystem/v3/pkg/writefs"
	"github.com/je4/filesystem/v3/pkg/zipfs"
	checksumImp "github.com/je4/utils/v2/pkg/checksum"
	"github.com/je4/utils/v2/pkg/zLogger"
	gocflCmd "github.com/ocfl-archive/gocfl/v2/gocfl/cmd"
	"github.com/ocfl-archive/ona/models"
	"github.com/ocfl-archive/ona/service"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	initialCopying = "initial copying"
	archived       = "archived"
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
	generateCmd.Flags().BoolP("wait", "w", false, "Wait until the order is finished")
	generateCmd.Flags().BoolP("force", "f", false, "Force to archive and retrieve checksum during the process")
}

func sendFile(cmd *cobra.Command, args []string) {
	wait, err := cmd.Flags().GetBool("wait")
	if err != nil {
		fmt.Println(err)
		return
	}
	cfgFilePath, err := cmd.Flags().GetString("config")
	if err != nil {
		fmt.Println(err)
		return
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("cannot get hostname: %v", err)
	}
	configObj := service.GetConfig(cfgFilePath)
	var loggerTLSConfig *tls.Config
	var loggerLoader io.Closer
	if configObj.Log.Stash.TLS != nil {
		loggerTLSConfig, loggerLoader, err = loader.CreateClientLoader(configObj.Log.Stash.TLS, nil)
		if err != nil {
			log.Fatalf("cannot create client loader: %v", err)
		}
		defer loggerLoader.Close()
	}
	_logger, _logstash, _logfile, err := ublogger.CreateUbMultiLoggerTLS(configObj.Log.Level, configObj.Log.File,
		ublogger.SetDataset(configObj.Log.Stash.Dataset),
		ublogger.SetLogStash(configObj.Log.Stash.LogstashHost, configObj.Log.Stash.LogstashPort, configObj.Log.Stash.Namespace, configObj.Log.Stash.LogstashTraceLevel),
		ublogger.SetTLS(configObj.Log.Stash.TLS != nil),
		ublogger.SetTLSConfig(loggerTLSConfig),
	)

	if err != nil {
		log.Fatalf("cannot create logger: %v", err)
	}

	l2 := _logger.With().Timestamp().Str("host", hostname).Logger() //.Output(output)
	var logger zLogger.ZLogger = &l2
	if _logstash != nil {
		defer _logstash.Close()
	}
	if _logfile != nil {
		defer _logfile.Close()
	}

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
	extension := filepath.Ext(filePathCleaned)

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
			logger.Error().Msgf("cannot close checksum writer", err)
			return
		}
		checksums, err := csWriter.GetChecksums()
		if err != nil {
			logger.Error().Msgf("cannot get checksum", err)
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

	objects, err := service.GetObjectsByChecksum(checksum, *configObj)
	if err != nil {
		logger.Error().Msgf("could not get objects from database to check whether object with checksum %s exists", checksum)
		return
	}

	if len(objects.Objects) != 0 {
		logger.Error().Msgf("The file with checksum: %s you are trying to archive already exists in archive\n", checksum)
		return
	}

	fsFactory, err := writefs.NewFactory()
	if err != nil {
		logger.Error().Msgf("cannot create filesystem factory", err)
		return
	}
	if err := fsFactory.Register(zipfs.NewCreateFSFunc(logger), "\\.zip$", writefs.HighFS); err != nil {
		logger.Error().Msgf("cannot register zipfs", err)
		return
	}
	if err := fsFactory.Register(osfsrw.NewCreateFSFunc(logger), "", writefs.LowFS); err != nil {
		logger.Error().Msgf("cannot register zipfs", err)
		return
	}
	extensionFactory, err := gocflCmd.InitExtensionFactory(map[string]string{},
		"",
		false,
		nil,
		nil,
		nil,
		nil,
		logger,
		"")
	if err != nil {
		logger.Error().Msgf("cannot instantiate extension factory", err)
		return
	}

	objectJson := ""
	object := models.Object{}
	if jsonPathRow != "" {
		jsonPathCleaned := filepath.ToSlash(filepath.Clean(jsonPathRow))
		jsonObject, err := os.ReadFile(jsonPathCleaned)
		if err != nil {
			logger.Error().Msgf("could not open json file: " + jsonPathCleaned)
			return
		}
		err = json.Unmarshal(jsonObject, &object)
		if err != nil {
			logger.Error().Msgf(err.Error())
			return
		}
		object.Checksum = checksum
		object.Size = objectSize
		ObjectJsonRaw, err := json.Marshal(object)
		if err != nil {
			logger.Error().Msgf(err.Error())
			return
		}
		objectJson = string(ObjectJsonRaw)
	} else {
		gocfl := service.NewGocfl(extensionFactory, fsFactory, logger)
		object, err = gocfl.ExtractMetadata(filePathCleaned)
		object.Checksum = checksum
		object.Size = objectSize
		if err != nil {
			logger.Error().Msgf("could not extract metadata for file: " + filePathCleaned)
			return
		}
		ObjectJsonRaw, err := json.Marshal(object)
		if err != nil {
			logger.Error().Msgf(err.Error())
			return
		}
		objectJson = string(ObjectJsonRaw)
	}
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]`)
	fileName := re.ReplaceAllString(object.Signature+extension, "_")

	status, err := service.GetStorageLocationsStatusForCollectionAlias(object.CollectionId, objectSize, *configObj)
	if err != nil {
		logger.Error().Msgf("could not get GetStorageLocationsStatusForCollectionAlias %s", err)
		return
	}
	if status != "" {
		logger.Error().Msgf(err.Error())
		return
	}

	objectInstances, err := service.GetObjectInstancesByName(fileName, *configObj)
	if err != nil {
		logger.Error().Msgf("could not get objectInstances from database to check whether file name %s exists", fileName)
		return
	}

	if len(objectInstances.ObjectInstances) != 0 {
		logger.Error().Msgf("The file: %s you are trying to archive already exists in archive\n", fileName)
		return
	}

	archivedStatus, err := service.CreateStatus(models.ArchivingStatus{Status: initialCopying}, *configObj)
	if err != nil {
		logger.Error().Msgf("could not create initial status")
		return
	}

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

	// create the tus client.
	url := configObj.Url
	client, err := tus.NewClient(url, &tus.Config{ChunkSize: configObj.ChunkSize, Header: map[string][]string{"Authorization": {configObj.Key},
		"ObjectJson": {objectJson}, "Collection": {object.CollectionId}, "StatusId": {archivedStatus.Id}, "Checksum": {checksum}, "FileName": {fileName}}, HttpClient: httpClient})
	if err != nil {
		logger.Error().Msgf("could not create client for: " + url)
		return
	}

	// create an upload from a file.
	upload, err := tus.NewUploadFromFile(file)
	if err != nil {
		logger.Error().Msgf("could not upload file: " + filePathCleaned)
		return
	}
	// create the uploader.
	uploader, err := client.CreateUpload(upload)
	if err != nil {
		logger.Error().Msgf("could not create upload for file: " + filePathCleaned + ", with err: " + err.Error())
		return
	}

	if !quiet {
		// start the uploading process.
		go func() {
			uploader.Upload()
		}()
		fmt.Println("Copy...")
		bar := progressbar.NewOptions64(
			upload.Size(),
			progressbar.OptionSetDescription(""),
			progressbar.OptionSetWriter(os.Stdout),
			progressbar.OptionSetWidth(10),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stdout, "\nUpload to temporary location is finished. Upload Id: "+archivedStatus.Id+" \n")
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
	if wait {
		for {
			archivedStatusW, err := service.GetStatus(archivedStatus.Id, *configObj)
			if err != nil {
				logger.Error().Msgf("could not get initial status with Id: " + archivedStatus.Id)
				return
			}
			if archivedStatusW.Status != archived {
				time.Sleep(10 * time.Second)
			} else {
				break
			}
		}

	}
}
