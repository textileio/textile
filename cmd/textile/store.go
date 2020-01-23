package main

import (
	"context"
	"errors"
	"os"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(storeCmd)
	storeCmd.AddCommand(
		addStoreCmd,
		lsStoreCmd,
		rmStoreCmd)
}

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Manage project files",
	Long:  `Manage your project's stored files.`,
}

var addStoreCmd = &cobra.Command{
	Use:   "add",
	Short: "Store a file",
	Long:  `Store a file in the active project.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		info, err := os.Stat(args[0])
		if err != nil {
			cmd.Fatal(err)
		}
		bar := pb.New(int(info.Size()))
		bar.SetTemplate(pb.Full)
		bar.Set(pb.Bytes, true)
		bar.Set(pb.SIBytesPrefix, true)
		bar.Start()
		progress := make(chan int64)
		go func() {
			for up := range progress {
				bar.SetCurrent(up)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()
		pth, err := client.Store(
			ctx,
			projectID,
			args[0],
			progress,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}
		bar.Finish()

		cmd.Success("Stored file at path: %s", aurora.White(pth.String()).Bold())
	},
}

var lsStoreCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List stored files",
	Long:  `List files stored in the active project.`,
	Run: func(c *cobra.Command, args []string) {
		//lsTokens()
	},
}

//func lsTokens() {
//	project := selectProject("Select project", aurora.Sprintf(
//		aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))
//
//	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
//	defer cancel()
//	tokens, err := client.ListAppTokens(
//		ctx,
//		project.ID,
//		api.Auth{
//			Token: authViper.GetString("token"),
//		})
//	if err != nil {
//		cmd.Fatal(err)
//	}
//
//	if len(tokens.List) > 0 {
//		data := make([][]string, len(tokens.List))
//		for i, t := range tokens.List {
//			data[i] = []string{t}
//		}
//		cmd.RenderTable([]string{"id"}, data)
//	}
//
//	cmd.Message("Found %d tokens", aurora.White(len(tokens.List)).Bold())
//}

var rmStoreCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Un-store a file",
	Long:  `Un-store a file in the active project (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		//project := selectProject("Select project", aurora.Sprintf(
		//	aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))
		//
		//selected := selectToken("Remove app token", aurora.Sprintf(
		//	aurora.BrightBlack("> Removing token {{ . | white | bold }}")),
		//	project.ID)
		//
		//ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		//defer cancel()
		//if err := client.RemoveAppToken(
		//	ctx,
		//	selected,
		//	api.Auth{
		//		Token: authViper.GetString("token"),
		//	}); err != nil {
		//	cmd.Fatal(err)
		//}
		//
		//cmd.Success("Removed app token %s", aurora.White(selected).Bold())
	},
}

//func selectToken(label, successMsg, projID string) string {
//	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
//	defer cancel()
//	tokens, err := client.ListAppTokens(
//		ctx,
//		projID,
//		api.Auth{
//			Token: authViper.GetString("token"),
//		})
//	if err != nil {
//		cmd.Fatal(err)
//	}
//
//	if len(tokens.List) == 0 {
//		cmd.End("You don't have any tokens!")
//	}
//
//	prompt := promptui.Select{
//		Label: label,
//		Items: tokens.List,
//		Templates: &promptui.SelectTemplates{
//			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ . | bold }}`, promptui.IconSelect),
//			Inactive: `{{ . | faint }}`,
//			Selected: successMsg,
//		},
//	}
//	index, _, err := prompt.Run()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	return tokens.List[index]
//}
