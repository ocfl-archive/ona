package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/je4/certloader/v2/pkg/loader"
	checksumImp "github.com/je4/utils/v2/pkg/checksum"
	"github.com/ocfl-archive/ona/models"
	"github.com/ocfl-archive/ona/service"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger"
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
	_logger, _logstash, _logfile := ublogger.CreateUbMultiLoggerTLS(configObj.Log.Level, configObj.Log.File,
		ublogger.SetDataset(configObj.Log.Stash.Dataset),
		ublogger.SetLogStash(configObj.Log.Stash.LogstashHost, configObj.Log.Stash.LogstashPort, configObj.Log.Stash.Namespace, configObj.Log.Stash.LogstashTraceLevel),
		ublogger.SetTLS(configObj.Log.Stash.TLS != nil),
		ublogger.SetTLSConfig(loggerTLSConfig),
	)
	/*
		if err != nil {
			log.Fatalf("cannot create logger: %v", err)
		}

	*/
	if _logstash != nil {
		defer _logstash.Close()
	}
	if _logfile != nil {
		defer _logfile.Close()
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		fmt.Println(err)
		return
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		fmt.Println(err)
		return
	}

	filePathRaw, _ := cmd.Flags().GetString("path")
	if filePathRaw == "" {
		fmt.Println("You should should specify path")
		return
	}
	filePathCleaned := filepath.ToSlash(filepath.Clean(filePathRaw))
	extension := filepath.Ext(filePathCleaned)

	file, err := os.Open(filePathCleaned)

	if err != nil {
		fmt.Println("could not open file: " + filePathRaw)
		return
	}
	defer file.Close()

	fileInfo, err := os.Stat(filePathCleaned)
	if err != nil {
		fmt.Println(err, "cannot read file: %v", err)
		return
	}
	objectSize := fileInfo.Size()

	jsonPathRow, err := cmd.Flags().GetString("json")
	if err != nil {
		fmt.Println(err)
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
			fmt.Println(err)
			return
		}
		if err := csWriter.Close(); err != nil {
			fmt.Println("cannot close checksum writer", err)
			return
		}
		checksums, err := csWriter.GetChecksums()
		if err != nil {
			fmt.Println("cannot get checksum", err)
		}
		checksum = checksums[checksumType]
	} else {
		fileChecksum, err := os.ReadFile(filePathCleaned + "." + checksumType)
		if err == nil {
			checksum = strings.Split(string(fileChecksum), separator)[0]
		} else {
			fmt.Println("You should have a checksum file in the folder or use -f flag to produce the checksum ")
			return
		}
	}

	objects, err := service.GetObjectsByChecksum(checksum, *configObj)
	if err != nil {
		fmt.Printf("could not get objects from database to check whether object with checksum %s exists", checksum)
		return
	}

	if len(objects.Objects) != 0 {
		fmt.Printf("The file with checksum: %s you are trying to archive allready exists in archive\n", checksum)
		return
	}

	objectJson := ""
	object := models.Object{}
	if jsonPathRow != "" {
		jsonPathCleaned := filepath.ToSlash(filepath.Clean(jsonPathRow))
		jsonObject, err := os.ReadFile(jsonPathCleaned)
		if err != nil {
			fmt.Println("could not open json file: " + jsonPathCleaned)
			return
		}
		err = json.Unmarshal(jsonObject, &object)
		if err != nil {
			fmt.Println(err)
			return
		}
		object.Checksum = checksum
		object.Size = objectSize
		ObjectJsonRaw, err := json.Marshal(object)
		if err != nil {
			fmt.Println(err)
			return
		}
		objectJson = string(ObjectJsonRaw)
	} else {
		object, err = service.ExtractMetadata(filePathCleaned, _logger.Logger)
		object.Checksum = checksum
		object.Size = objectSize
		if err != nil {
			fmt.Println("could not extract metadata for file: " + filePathCleaned)
			return
		}
		ObjectJsonRaw, err := json.Marshal(object)
		if err != nil {
			fmt.Println(err)
			return
		}
		objectJson = string(ObjectJsonRaw)
	}
	re := regexp.MustCompile(`[^-_.a-zA-Z0-9]`)
	fileName := re.ReplaceAllString(object.Signature+extension, "_")

	status, err := service.GetStorageLocationsStatusForCollectionAlias(object.CollectionId, objectSize, *configObj)
	if err != nil {
		fmt.Printf("could not get GetStorageLocationsStatusForCollectionAlias %s", err)
		return
	}
	if status != "" {
		fmt.Printf(err.Error())
		return
	}

	objectInstances, err := service.GetObjectInstancesByName(fileName, *configObj)
	if err != nil {
		fmt.Printf("could not get objectInstances from database to check whether file name %s exists", fileName)
		return
	}

	if len(objectInstances.ObjectInstances) != 0 {
		fmt.Printf("The file: %s you are trying to archive allready exists in archive\n", fileName)
		return
	}

	archivedStatus, err := service.CreateStatus(models.ArchivingStatus{Status: initialCopying}, *configObj)
	if err != nil {
		fmt.Println("could not create initial status")
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
		fmt.Println("could not create client for: " + url)
		return
	}

	// create an upload from a file.
	upload, err := tus.NewUploadFromFile(file)
	if err != nil {
		fmt.Println("could not upload file: " + filePathCleaned)
		return
	}
	// create the uploader.
	uploader, err := client.CreateUpload(upload)
	if err != nil {
		fmt.Println("could not create upload for file: " + filePathCleaned + ", with err: " + err.Error())
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
				fmt.Println("could not get initial status with Id: " + archivedStatus.Id)
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
