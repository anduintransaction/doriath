// Copyright © 2018 Anduin Transactions Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/anduintransaction/doriath/buildtree"
	"github.com/anduintransaction/doriath/utils"
	"github.com/spf13/cobra"
)

// findlatestCmd represents the findlatest command
var findlatestCmd = &cobra.Command{
	Use:   "findlatest image-name",
	Short: "Find latest tag from an image name",
	Long: `Find latest tag from an image name.

This command will find the tag with the same hash as 'latest' tag. If there is no
such tag, an error will be thrown.

Currently, only images hosted in gcr.io are supported by this command.
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		t, err := buildtree.ReadBuildTreeFromFile(cfgFile, variableMap, variableFiles)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		tag, err := t.FindLatestTag(args[0])
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		fmt.Println(tag)
	},
}

func init() {
	RootCmd.AddCommand(findlatestCmd)
}
