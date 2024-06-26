package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"gitlab.switch.ch/ub-unibas/dlza/ona/service"
)

const colorRed = "\033[0;31m"
const colorGreen = "\033[1;32m"
const colorNone = "\033[0m"

var generateCmdStored = &cobra.Command{
	Use:   "stored",
	Short: "Check whether/how file is stored",
	Long: `Check whether/how file is stored.
	For example:
	ona stored -n test_file_ub.zip -c C:\Users\config.yml
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: checkStorage,
}

func init() {
	rootCmd.AddCommand(generateCmdStored)
	generateCmdStored.Flags().StringP("name", "n", "", "name of file to be checked")
}

func checkStorage(cmd *cobra.Command, args []string) {
	cfgFilePath, err := cmd.Flags().GetString("config")
	if err != nil {
		fmt.Println(err)
		return
	}
	configObj := service.GetConfig(cfgFilePath)
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		fmt.Println("You should should specify name")
		return
	}
	objectInstances, err := service.GetObjectInstancesByName(name, *configObj)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(objectInstances.ObjectInstances) == 0 {
		fmt.Printf("File with name %v is stored on %v storage locations\n", name, len(objectInstances.ObjectInstances))
	} else {
		resultingQualityPb, err := service.GetQualityForObject(objectInstances.ObjectInstances[0].ObjectId, service.ResultingQuality, *configObj)
		if err != nil {
			fmt.Println(err)
			return
		}
		neededQualityPb, err := service.GetQualityForObject(objectInstances.ObjectInstances[0].ObjectId, service.NeededQuality, *configObj)
		if err != nil {
			fmt.Println(err)
			return
		}
		resultingQuality := resultingQualityPb.Size
		qualityNeeded := neededQualityPb.Size
		color := ""
		if resultingQualityPb.Size >= qualityNeeded {
			color = colorGreen
		} else {
			color = colorRed
		}
		fmt.Printf("File with name %v is stored on %v storage locations %v with quality %v%v. The lowest quality needed: %v\n", name, len(objectInstances.ObjectInstances), color, resultingQuality, colorNone, qualityNeeded)
	}
}
