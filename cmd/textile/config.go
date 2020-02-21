package main

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/cmd"
)

func init() {
	listConfigsCmd.Flags().StringP("scope", "s", "prod", "specify the scope for which to list config values (dev, beta, prod)")

	saveConfigCmd.Flags().StringP("scope", "s", "prod", "specify the scope for which to save the value (dev, beta, prod)")

	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(
		listConfigsCmd,
		getConfigCmd,
		saveConfigCmd,
		deleteConfigCmd,
	)
}

var configCmd = &cobra.Command{
	Use: "config",
	Aliases: []string{
		"conf",
	},
	Short: "Manage your project's configuration values",
	Long:  `Manage your project's configuration values`,
}

var listConfigsCmd = &cobra.Command{
	Use: "list",
	Aliases: []string{
		"ls",
	},
	Short: "List the config itmes for the current project",
	Long:  `List the config itmes for the current project`,
	Run: func(c *cobra.Command, args []string) {
		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()

		configItems, err := client.ListConfigItems(ctx, project, api.Auth{
			Token: authViper.GetString("token"),
		})
		if err != nil {
			cmd.Fatal(err)
		}

		scope := c.Flag("scope").Value.String()

		data := make([][]string, len(configItems))
		for i, configItem := range configItems {
			data[i] = []string{
				configItem.GetName(),
				configItem.GetValues()[scope],
			}
		}

		cmd.Message("Config items for scope %v:", scope)
		c.Println()
		cmd.RenderTable([]string{"name", "value"}, data)
	},
}

var getConfigCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a single config item in the current project",
	Long:  `Get a single config item in the current project`,
	Run: func(c *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Fatal(errors.New("you must provide one arguments, the config item name to inspect"))
		}

		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()

		configItem, err := client.GetConfigItem(ctx, project, args[0], api.Auth{
			Token: authViper.GetString("token"),
		})
		if err != nil {
			cmd.Fatal(err)
		}

		data := make([][]string, len(configItem.GetValues()))
		i := 0
		for scope, value := range configItem.GetValues() {
			data[i] = []string{
				scope,
				value,
			}
			i++
		}

		cmd.RenderTable([]string{"scope", "value"}, data)
	},
}

var saveConfigCmd = &cobra.Command{
	Use:   "save [name] [value]",
	Short: "Save a config item for the current project",
	Long:  `Save a config item for the current project`,
	Run: func(c *cobra.Command, args []string) {
		if len(args) != 2 {
			cmd.Fatal(errors.New("you must provide two arguments, the config item name and value"))
		}

		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()

		scope := c.Flag("scope").Value.String()

		values := map[string]string{scope: args[1]}
		payloads := []*pb.SaveConfigItemsRequest_Payload{
			&pb.SaveConfigItemsRequest_Payload{
				Name:   args[0],
				Values: values,
			},
		}

		_, err := client.SaveConfigItems(ctx, project, payloads, api.Auth{
			Token: authViper.GetString("token"),
		})
		if err != nil {
			cmd.Fatal(err)
		}
	},
}

var deleteConfigCmd = &cobra.Command{
	Use: "delete",
	Aliases: []string{
		"rm",
	},
	Short: "Delete a config item in the current project",
	Long:  `Delete a config item in the current project`,
	Run: func(c *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Fatal(errors.New("you must provide one arguments, the config item name to delete"))
		}

		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()

		err := client.DeleteConfigItem(ctx, project, args[0], api.Auth{
			Token: authViper.GetString("token"),
		})
		if err != nil {
			cmd.Fatal(err)
		}
	},
}
