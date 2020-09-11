package cli

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/caarlos0/spin"
	"github.com/logrusorgru/aurora"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	bi "github.com/textileio/textile/buildinfo"
	"github.com/textileio/textile/cmd"
)

func checkProduction() (*su.Release, error) {
	s := spin.New("%s Checking latest Hub CLI release")
	s.Start()
	defer s.Stop()
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
	s := spin.New("%s Checking Hub API version")
	s.Start()
	defer s.Stop()
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
			cmd.Warn("Unable to get latest release.")
		} else {
			current, err := semver.ParseTolerant(version)
			if err != nil {
				// Display warning if off production
				cmd.Warn("Running a custom hub build. Run `%s` to install the latest release.", aurora.White("hub update").Bold())
			} else if current.LT(latest.Version) {
				// Display warning if outdated
				cmd.Warn("There is a new hub release. Run `%s` to install %s.", aurora.White("hub update").Bold(), aurora.Cyan(latest.Version.String()))
			}
		}

		if version == "git" {
			cmd.Message("Custom version.")
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
