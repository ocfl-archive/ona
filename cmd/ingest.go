package cmd

import (
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/joho/godotenv"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"ona/configuration"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	// Relative on runtime DIR:
	_, b, _, _ := runtime.Caller(0)
	d1 := strings.Replace(filepath.ToSlash(path.Join(path.Dir(b))), "/cmd", "", -1)
	err := godotenv.Load(d1 + "/.env")
	if err != nil {
		fmt.Println(err)
	}
	configObj := &configuration.Config{
		Url: os.Getenv("URL"),
		Key: os.Getenv("KEY"),
	}
	chunkSize, _ := strconv.Atoi(os.Getenv("CHUNK_SIZE"))
	configObj.ChunkSize = int64(chunkSize)
	configObj.BarPause, _ = strconv.Atoi(os.Getenv("BAR_PAUSE"))

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

	// create the tus client.
	url := configObj.Url
	client, err := tus.NewClient(url, &tus.Config{ChunkSize: configObj.ChunkSize, Header: map[string][]string{"Authorization": {configObj.Key}}})
	if err != nil {
		fmt.Println("could not create client for: " + url)
		return
	}

	jsonPathRow, _ := cmd.Flags().GetString("json")
	if jsonPathRow != "" {
		jsonPathCleaned := filepath.ToSlash(filepath.Clean(jsonPathRow))
		json, err := os.Open(jsonPathCleaned)

		if err != nil {
			fmt.Println("could not open json file: " + jsonPathCleaned)
			return
		}
		// create an upload from a file.
		uploadJson, err := tus.NewUploadFromFile(json)
		// create the uploader.
		uploader, err := client.CreateUpload(uploadJson)
		if err != nil {
			fmt.Println("could not create upload for file: " + filePathCleaned)
			return
		}
		// start the uploading process.
		uploader.Upload()
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
		fmt.Println("could not create upload for file: " + filePathCleaned)
		return
	}

	if !quiet {
		// start the uploading process.
		go func() {
			uploader.Upload()
		}()
		fmt.Println("Copy...")
		amount := 0
		bar := progressbar.Default(100)
		for {
			offset := upload.Offset()
			size := upload.Size()
			percent := int((float32(offset) / float32(size)) * 100)
			difference := percent - amount
			amount = percent
			bar.Add(difference)
			if offset == size {
				fmt.Println("Upload is finished")
				break
			}
			time.Sleep(time.Duration(configObj.BarPause) * time.Millisecond)
		}
	} else {
		uploader.Upload()
	}
}
