package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"ona/service"
)

var generateCmdStatus = &cobra.Command{
	Use:   "status",
	Short: "Get status",
	Long: `Receive status of copying process.
	For example:
	ona status -i 1a11f892-e94b-47da-89d3-ceee985e0d8c
	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: getStatus,
}

func init() {
	rootCmd.AddCommand(generateCmdStatus)
	generateCmdStatus.Flags().StringP("id", "i", "", "Id of copying process")
}

func getStatus(cmd *cobra.Command, args []string) {
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		fmt.Println("You should should specify id")
		return
	}
	status, err := service.GetStatus(id)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(status.Status)
}
