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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cfgFile string
var variableArray []string
var variableMap map[string]string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "doriath",
	Short: "A simple tool to manage docker build graph",
	Long:  `A simple tool to manage docker build graph`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		for _, variable := range variableArray {
			segments := strings.SplitN(variable, "=", 2)
			if len(segments) != 2 {
				fmt.Fprintf(os.Stderr, "Invalid variable: %s\n", variable)
				os.Exit(2)
			}
			variableMap[segments[0]] = segments[1]
		}
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "doriath.yml", fmt.Sprint("config file (default is 'doriath.yml' in current folder)"))
	variableMap = make(map[string]string)
	RootCmd.PersistentFlags().StringArrayVarP(&variableArray, "variable", "x", []string{}, "variables to pass to config file")
}
