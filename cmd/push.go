// Copyright © 2017 Anduin Transactions Inc
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"os"

	"github.com/anduintransaction/doriath/buildtree"
	"github.com/anduintransaction/doriath/utils"
	"github.com/spf13/cobra"
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push all new images to docker registry",
	Long:  "Push all new images to docker registry",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := buildtree.ReadBuildTreeFromFile(cfgFile, variableMap, variableFiles)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		err = t.Prepare()
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		err = t.Push()
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(pushCmd)
}
