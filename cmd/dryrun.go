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

var (
	dryrunNoColor       = false
	printSkipDirtyCheck = false
)

// dryrunCmd represents the dryrun command
var dryrunCmd = &cobra.Command{
	Use:   "dryrun",
	Short: "Check your project for build steps and possible error",
	Long:  "Check your project for build steps and possible error",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := buildtree.ReadBuildTreeFromFile(cfgFile, variableMap, variableFiles)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		opts := []buildtree.PrepareOptFn{}
		if printSkipDirtyCheck {
			opts = append(opts, buildtree.SkipDirtyCheck())
		}
		err = t.Prepare(opts...)
		if err != nil {
			utils.Error(err)
			os.Exit(1)
		}
		t.PrintTree(dryrunNoColor)
	},
}

func init() {
	RootCmd.AddCommand(dryrunCmd)
	dryrunCmd.Flags().BoolVar(&printSkipDirtyCheck, "skip-check", false, "Skip dirty check")
	dryrunCmd.Flags().BoolVarP(&dryrunNoColor, "no-color", "c", false, "No color output")
}
