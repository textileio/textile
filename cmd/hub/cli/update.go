package cli

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/caarlos0/spin"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	bi "github.com/textileio/textile/v2/buildinfo"
	"github.com/textileio/textile/v2/cmd"
)

func install(assetURL string) error {
	s := spin.New("%s Downloading release")
	s.Start()
	defer s.Stop()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := su.UpdateTo(assetURL, exe); err != nil {
		return err
	}
	return nil
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hub",
	Long:  `Update hub to the latest version.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		latestRelease, err := getLatestRelease()
		if err != nil {
			cmd.Fatal(fmt.Errorf("unable to get latest version: %v", err))
		}
		latest := "v" + latestRelease.Version.String()

		current, err := semver.ParseTolerant(bi.Version)
		if err != nil {
			// Overwrite local build
			if err := install(latestRelease.AssetURL); err != nil {
				cmd.Fatal(fmt.Errorf("install failed: %v", err))
			} else {
				cmd.Success("hub updated to %s", aurora.Green(latest))
			}
		} else {
			// Install if not up-to-date
			if current.LT(latestRelease.Version) {
				if err = install(latestRelease.AssetURL); err != nil {
					cmd.Fatal(fmt.Errorf("install failed: %v", err))
				} else {
					cmd.Success("hub updated to %s", aurora.Green(latest))
				}
			} else {
				cmd.Message("Already up-to-date.")
			}
		}
	},
}
