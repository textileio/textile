package cli

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/logrusorgru/aurora"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	bi "github.com/textileio/textile/buildinfo"
	"github.com/textileio/textile/cmd"
)

func checkProduction() (*su.Release, error) {
	config := su.Config{
		Filters: []string{
			"hub",
		},
	}
	updater, err := su.NewUpdater(config)
	if err != nil {
		return nil, err
	}

	latest, found, err := updater.DetectLatest(cmd.Repo)
	if err != nil || !found {
		return nil, err
	}
	return latest, nil
}

func getAPIVersion() (string, error) {
	ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
	defer cancel()
	res, err := clients.Hub.BuildInfo(ctx)
	if err != nil {
		return "", err
	}
	if res.Version == "development" {
		return fmt.Sprintf("%s (%s)", res.GitCommit, res.BuildDate), nil
	}
	return res.Version, nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version",
	Long:  `Shows the installed CLI version.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		version := bi.Version

		apiVersion, err := getAPIVersion()
		if err != nil {
			cmd.Error(err)
			cmd.Warn("Unable to check API version.")
		} else {
			cmd.Message("The Hub API is running %s", apiVersion)
		}

		latest, err := checkProduction()
		if err != nil {
			cmd.Error(err)
			cmd.Warn("Unable to check latest public release.")
		} else {
			current, err := semver.ParseTolerant(version)
			if err != nil {
				// Display warning if off production
				cmd.Warn("Running a developer branch. Run `%s` to install production release.", aurora.White("hub update").Bold())
			} else if current.LT(latest.Version) {
				// Display warning if outdated
				cmd.Warn("Your hub is behind. Run `%s` to install %s.", aurora.White("hub update").Bold(), aurora.Cyan(latest.Version.String()))
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
