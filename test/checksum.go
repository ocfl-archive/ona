package main

import (
	"bytes"
	"fmt"
	"github.com/je4/utils/v2/pkg/checksum"
	"io"
	"sync"
)

func main() {
	var data = []byte(`"2023-10-01T12:00:00Z"`)

	var reader io.Reader = bytes.NewReader(data)

	p0r, p0w := io.Pipe()

	csWriter, err := checksum.NewChecksumWriter([]checksum.DigestAlgorithm{checksum.DigestSHA512},
		p0w)
	if err != nil {
		panic(err)
	}

	var wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer csWriter.Close()
		defer p0w.Close()
		// reader.Close()
		defer wg.Done()
		if _, err := io.Copy(csWriter, reader); err != nil {
			panic(err)
		}
	}()

	target := bytes.NewBuffer(nil)

	if _, err := io.Copy(target, p0r); err != nil {
		panic(err)
	}
	wg.Wait()
	fmt.Println("Data copied to target buffer:", target.String())
	cs, err := csWriter.GetChecksums()
	if err != nil {
		panic(err)
	}
	sha512, ok := cs[checksum.DigestSHA512]
	if !ok {
		panic("SHA512 checksum not found")
	}
	fmt.Println("SHA512 Checksum:", sha512)
}

/*
-----------------
Fall 1:
data.zip
data.sha512
data.json

Upload:
StartUpload: Binary: yes/no ---> UploadID
1. data.zip Header: UploadID
2. data.json  Header: UploadID
teste Prüfsumme
CompleteUpload: ---> UploadID, Checksum
DiscardUpload: ---> UploadID

-----------------
Fall 2:
data.zip
data.json

Upload:
StartUpload: ---> UploadID
1. data.zip Header: UploadID
2. data.json  Header: UploadID
warne, dass data.sha512 fehlt
CompleteUpload: ---> UploadID, Checksum

-----------------
Fall 3:
data.zip

Upload:
StartUpload: ---> UploadID
1. data.zip Header: UploadID
2. data.json extrahiert aus data.zip  Header: UploadID
CompleteUpload: ---> UploadID, Checksum




*/
