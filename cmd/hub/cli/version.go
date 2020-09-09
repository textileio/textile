package cli

import (
	"os"

	"github.com/blang/semver"
	"github.com/caarlos0/spin"
	"github.com/logrusorgru/aurora"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version",
	Long:  `Shows the installed CLI version.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		version := cmd.Version

		update, err := c.Flags().GetBool("update")
		cmd.ErrCheck(err)

		current, err := semver.ParseTolerant(version)
		if err == nil {
			config := su.Config{
				Filters: []string{
					"hub",
				},
			}
			updater, err := su.NewUpdater(config)
			if err != nil {
				cmd.Fatal(err)
				return
			}

			latest, found, err := updater.DetectLatest(cmd.Repo)
			if err != nil {
				cmd.Fatal(err)
				return
			}

			if found && err == nil {
				if current.LT(latest.Version) {
					if update {
						// Update cli if requested
						exe, err := os.Executable()
						if err != nil {
							cmd.Fatal(err)
							return
						}

						s := spin.New("%s Downloading release")
						s.Start()
						if err := su.UpdateTo(latest.AssetURL, exe); err != nil {
							cmd.Fatal(err)
							return
						}
						version = latest.Version.String()
						s.Stop()

						cmd.Message("Success: hub updated.")
					} else {
						// Display warning if outdated
						cmd.Message("Warning: hub is behind. Run `%s` to install %s.", aurora.White("hub version --update").Bold(), aurora.Cyan(latest.Version.String()))
					}
				} else if update {
					cmd.Message("Info: hub already up-to-date.")
				}
			}
		}
		cmd.Message("%s", aurora.Green(version))
	},
}
