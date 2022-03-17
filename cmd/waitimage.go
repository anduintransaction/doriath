package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/anduintransaction/doriath/buildtree"
	"github.com/anduintransaction/doriath/utils"
	"github.com/spf13/cobra"
)

var (
	waitTimeout  time.Duration
	waitInterval time.Duration
)

// findlatestCmd represents the findlatest command
var waitimageCmd = &cobra.Command{
	Use:   "waitimage image-name",
	Short: "Wait until an image exist in registry",
	Long: `Wait until an image exist in registry.

This command will try to check if the specified tag is in tag list. If there is no such tag, it will wait
for an interval before retry again. If timeout exceeded, it will stop retry and exit 1.

Currently, only images hosted in gcr.io are supported by this command.
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		t, err := buildtree.ReadBuildTreeFromFile(cfgFile, variableMap, variableFiles)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		err = t.WaitImageExist(args[0], waitTimeout, waitInterval)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		fmt.Println("OK")
	},
}

func init() {
	waitimageCmd.PersistentFlags().DurationVarP(&waitTimeout, "timeout", "t", 5*time.Minute, "Wait timeout before exit")
	waitimageCmd.PersistentFlags().DurationVarP(&waitInterval, "interval", "i", time.Second, "Interval between retry")
	RootCmd.AddCommand(waitimageCmd)
}
