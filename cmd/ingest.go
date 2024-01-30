package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"ona/models"
	"ona/service"
	"os"
	"path/filepath"
	"time"
)

var generateCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Send files to storage",
	Long: `Send files to storage.
	For example:
	ona ingest -q -p 123-345.zip
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: sendFile,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("json", "j", "", "Path to json file")
	generateCmd.Flags().StringP("path", "p", "", "Path to file")
	generateCmd.Flags().BoolP("quiet", "q", false, "Should the process information be showed")
}

func sendFile(cmd *cobra.Command, args []string) {
	configObj := service.GetConfig()
	quiet, _ := cmd.Flags().GetBool("quiet")

	filePathRow, _ := cmd.Flags().GetString("path")
	if filePathRow == "" {
		fmt.Println("You should should specify path")
		return
	}
	filePathCleaned := filepath.ToSlash(filepath.Clean(filePathRow))
	file, err := os.Open(filePathCleaned)

	if err != nil {
		fmt.Println("could not open file: " + filePathRow)
		return
	}
	defer file.Close()

	jsonPathRow, _ := cmd.Flags().GetString("json")

	objectJson := ""
	object := models.Object{}
	if jsonPathRow != "" {
		jsonPathCleaned := filepath.ToSlash(filepath.Clean(jsonPathRow))
		jsonObject, err := os.ReadFile(jsonPathCleaned)
		if err != nil {
			fmt.Println("could not open json file: " + jsonPathCleaned)
			return
		}
		_ = json.Unmarshal(jsonObject, &object)
		ObjectJsonRaw, _ := json.Marshal(object)
		objectJson = string(ObjectJsonRaw)
	} else {
		objectMeta, err := service.ExtractMetadata(filePathCleaned)
		if err != nil {
			fmt.Println("could not extract metadata for file: " + filePathCleaned)
			return
		}
		ObjectJsonRaw, _ := json.Marshal(objectMeta)
		_ = json.Unmarshal(ObjectJsonRaw, &object)
		objectJson = string(ObjectJsonRaw)
	}

	// create the tus client.
	url := configObj.Url
	client, err := tus.NewClient(url, &tus.Config{ChunkSize: configObj.ChunkSize, Header: map[string][]string{"Authorization": {configObj.Key}, "ObjectJson": {objectJson}, "Collection": {object.Collection}}})
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
				fmt.Fprint(os.Stdout, "\nUpload finished\n")
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
