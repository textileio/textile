package cli

import (
	"os"

	"github.com/blang/semver"
	"github.com/caarlos0/spin"
	"github.com/logrusorgru/aurora"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	bi "github.com/textileio/textile/buildinfo"
	"github.com/textileio/textile/cmd"
)

func install(assetURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s := spin.New("%s Downloading release")
	s.Start()
	if err := su.UpdateTo(assetURL, exe); err != nil {
		return err
	}
	s.Stop()
	return nil
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the hub CLI",
	Long:  `Update the installed hub CLI version to latest release.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		version := bi.Version

		latest, err := checkProduction()
		if err != nil {
			cmd.Error(err)
			cmd.Warn("Unable to check latest public release.")
		} else {
			current, err := semver.ParseTolerant(version)
			if err == nil {
				if current.LT(latest.Version) {
					if err = install(latest.AssetURL); err != nil {
						cmd.Error(err)
						cmd.Warn("Error: install failed.")
					} else {
						version = latest.Version.String()
						cmd.Message("Success: hub updated.")
					}
				} else {
					cmd.Message("Already up-to-date.")
				}
			} else {
				if err = install(latest.AssetURL); err != nil {
					cmd.Error(err)
					cmd.Warn("Error: install failed.")
				} else {
					version = latest.Version.String()
					cmd.Message("Success: hub updated.")
				}
			}
		}
		if version == "git" {
			cmd.RenderTable(
				[]string{"GitBranch", "GitState", "GitSummary"},
				[][]string{{
					bi.GitBranch,
					bi.GitState,
					bi.GitSummary,
				}},
			)
			cmd.Message("%s (%s)", aurora.Green(bi.GitCommit), bi.BuildDate)
		} else {
			cmd.Message("%s", aurora.Green(version))
		}
	},
}
