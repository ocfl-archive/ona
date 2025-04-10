package cmd

import (
	"crypto/tls"
	"fmt"
	"github.com/je4/filesystem/v3/pkg/vfsrw"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/ocfl-archive/ona/service"
	"github.com/spf13/cobra"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy files from storage",
	Long: `Copy files from storage. A signature should be provided.
	For example:
	ona copy -s alma1234 -p C:\Users -c C:\Users\config.yml
	will copy alma1234 toC:\Users folder.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: copyFile,
}

func init() {
	rootCmd.AddCommand(copyCmd)
	copyCmd.Flags().StringP("path", "p", "", "Path to folder to copy in")
	copyCmd.Flags().StringP("signature", "s", "", "signature of file")
}

func copyFile(cmd *cobra.Command, args []string) {
	cfgFilePath, err := cmd.Flags().GetString("config")
	if err != nil {
		fmt.Println(err)
		return
	}
	signature, err := cmd.Flags().GetString("signature")
	if err != nil {
		fmt.Println(err)
		return
	}
	path, err := cmd.Flags().GetString("path")
	if err != nil {
		fmt.Println(err)
		return
	}
	configObj := service.GetConfig(cfgFilePath)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("cannot get hostname: %v", err)
	}

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

	objectInstance, err := service.GetObjectInstancesBySignatureAndLocationsPathName(signature, *configObj)
	if err != nil {
		logger.Panic().Msgf("error extracting object instance with signature: %s", signature, err)
		return
	}

	vfsConfig, err := service.LoadVfsConfig(*configObj)
	if err != nil {
		logger.Panic().Msgf("error mapping json for storage location connection field: %v", err)
		return
	}

	vfs, err := vfsrw.NewFS(vfsConfig, &l2)
	if err != nil {
		logger.Panic().Err(err).Msg("cannot create vfs")
		return
	}
	defer func() {
		if err := vfs.Close(); err != nil {
			logger.Error().Err(err).Msg("cannot close vfs")
		}
	}()
	sourceFP, err := vfs.Open(objectInstance.Path)
	if err != nil {
		logger.Panic().Msgf("cannot read file '%s': %v", signature, err)
		return
	}
	defer func() {
		if err := sourceFP.Close(); err != nil {
			logger.Error().Msgf("cannot close source: %v", err)
		}
	}()
	signature = strings.Replace(signature, ":", "_", -1)
	fullPath := filepath.ToSlash(filepath.Clean(fmt.Sprintf("%s/%s.zip", path, signature)))
	destination, err := os.Create(fullPath)
	if err != nil {
		logger.Panic().Msgf("cannot create destination for file '%s%s': %v", signature, path, err)
		return
	}
	logger.Info().Msgf("Copying...")
	written, err := io.Copy(destination, sourceFP)
	if err != nil {
		logger.Panic().Msgf("cannot copy file '%s%s': %v", signature, path, err)
		return
	}
	logger.Info().Msgf("File %s with size %d bytes was copied. %s", signature, written, fullPath)
}
