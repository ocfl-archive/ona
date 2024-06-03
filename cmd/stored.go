package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"ona/service"
)

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
	fmt.Printf("File with name %v is stored on %v storage locations\n", name, len(objectInstances.ObjectInstances))
}
