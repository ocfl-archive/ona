package cmd

import (
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var generateCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Send files to storage",
	Long: `Send files to storage.
	For example:
	ona ingest -p 123-345.zip
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: sendFile,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringP("path", "p", "", "Path to file")
}

func sendFile(cmd *cobra.Command, args []string) {
	filePath, _ := cmd.Flags().GetString("path")
	if filePath == "" {
		fmt.Println("You should should specify path")
		return
	}
	file, err := os.Open(filePath)

	if err != nil {
		fmt.Println("could not open file: " + filePath)
		return
	}

	defer file.Close()

	// create the tus client.
	url := "http://localhost:8085/files/"
	client, err := tus.NewClient(url, &tus.Config{ChunkSize: 100000000, Header: map[string][]string{"Authorization": {"testHeader"}}})
	if err != nil {
		fmt.Println("could not create client for: " + url)
		return
	}

	// create an upload from a file.
	upload, err := tus.NewUploadFromFile(file)
	if err != nil {
		fmt.Println("could not upload file: " + filePath)
		return
	}

	// create the uploader.
	uploader, err := client.CreateUpload(upload)
	if err != nil {
		fmt.Println("could not create upload for file: " + filePath)
		return
	}

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
		time.Sleep(25 * time.Millisecond)
	}
}
