package cli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/blang/semver"
	"github.com/caarlos0/spin"
	su "github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	bi "github.com/textileio/textile/v2/buildinfo"
	"github.com/textileio/textile/v2/cmd"
)

func getLatestRelease() (*su.Release, error) {
	s := spin.New("%s Checking for new Hub CLI version")
	s.Start()
	defer s.Stop()
	config := su.Config{
		Filters: []string{
			"hub_v",
		},
	}
	updater, err := su.NewUpdater(config)
	if err != nil {
		return nil, err
	}

	latest, found, err := updater.DetectLatest(cmd.Repo)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("%s_%s build not found", runtime.GOOS, runtime.GOARCH)
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
		return "", fmt.Errorf("getting API build info: %v", err)
	}
	if res.GitSummary == "" {
		res.GitSummary = "git"
	}
	return res.GitSummary, nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version info",
	Long:  `Shows version info.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		isGovvvBuild := bi.GitSummary != ""
		isReleaseBuild := bi.Version != "git"

		displayVersion := bi.Version
		if !isReleaseBuild && isGovvvBuild {
			displayVersion = bi.GitSummary
		}
		cmd.Message("%s", aurora.Green(displayVersion))

		apiVersion, err := getAPIVersion()
		if err != nil {
			cmd.Warn("Unable to get Hub API version: %v", err)
		} else {
			cmd.Message("Hub API: %s", apiVersion)
		}

		latestRelease, err := getLatestRelease()
		if err != nil {
			cmd.Warn("Unable to get latest version: %v", err)
		} else {
			latest := "v" + latestRelease.Version.String()
			current, err := semver.ParseTolerant(bi.Version)
			if err != nil || current.LT(latestRelease.Version) {
				// Display message if outdated of we're running a custom build
				cmd.Message("New version of hub available! %s -> %s. Run %s to update.",
					aurora.Red(displayVersion),
					aurora.Cyan(latest),
					aurora.White("hub update").Bold())
			}
		}
	},
}
