package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init",
	Long:  `Initialize a new project.`,
	Run: func(c *cobra.Command, args []string) {
		fmt.Println("todo")
	},
}
