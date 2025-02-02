package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/fatih/color"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var rootFlags struct {
	Debug     bool
	Directory string
}

var rootCmd = &cobra.Command{
	Use: "av",

	// Don't automatically print errors or usage information (we handle that ourselves).
	// Cobra still prints usage if you return cmd.Usage() from RunE.
	SilenceErrors: true,
	SilenceUsage:  true,

	// Don't show "completion" command in help menu
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},

	// Run setup before invoking any child commands.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if rootFlags.Debug {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.WithField("av_version", config.Version).Debug("enabled debug logging")
		}

		var configDirs []string
		repo, err := getRepo()
		// If we weren't able to load the Git repo, that probably just means the
		// command isn't being run from inside a repo. That's fine, we just
		// don't need to bother reading repo-local config.
		if err != nil {
			logrus.WithError(err).Debug("unable to load Git repo (probably not inside a repo)")
		} else {
			gitDir, err := repo.Git("rev-parse", "--git-dir")
			if err != nil {
				logrus.WithError(err).Warning("failed to determine git root directory")
			} else {
				configDirs = append(configDirs, gitDir)
			}
			logrus.WithField("git_dir", gitDir).Debug("loaded Git repo")
		}

		// Note: this only returns an error if config exists and it can't be
		// read/parsed. It doesn't return an error if no config file exists.
		didLoadConfig, err := config.Load(configDirs)
		if err != nil {
			return errors.Wrap(err, "failed to load configuration")
		}
		if didLoadConfig {
			logrus.Debug("loaded configuration")
		} else {
			logrus.Debug("no configuration found")
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(
		&rootFlags.Debug, "debug", false,
		"enable verbose debug logging",
	)
	rootCmd.PersistentFlags().StringVarP(
		&rootFlags.Directory, "repo", "C", "",
		"directory to use for git repository",
	)
	rootCmd.AddCommand(
		fetchCmd,
		initCmd,
		prCmd,
		stackCmd,
		versionCmd,
	)
}

// errExitSilently is an error type that indicates that program should exit
// without printing any additional information with the given exit code.
// This is meant for cases where the running commands wants to manage its own
// error output but still needs to return a non-zero exit code (since returning
// nil from RunE would cause a exit with a zero code).
type errExitSilently struct {
	exitCode int
}

func (e errExitSilently) Error() string {
	return "<exit silently>"
}

func main() {
	// Note: this doesn't include whatever time is spent in initializing the
	// runtime and various packages (e.g., package init functions).
	startTime := time.Now()
	err := rootCmd.Execute()
	logrus.WithField("duration", time.Since(startTime)).Debug("command exited")
	checkCliVersion()
	var exitSilently errExitSilently
	if errors.As(err, &exitSilently) {
		os.Exit(exitSilently.exitCode)
	}
	if err != nil {
		// In debug mode, show more detailed information about the error
		// (including the stack trace if using pkg/errors).
		if rootFlags.Debug {
			stackTrace := fmt.Sprintf("%+v", err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %s\n%s\n", err, text.Indent(stackTrace, "\t"))
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}

func checkCliVersion() {
	if config.Version == config.VersionDev {
		logrus.Debug("skipping CLI version check (development version)")
		return
	}
	latest, err := config.FetchLatestVersion()
	if err != nil {
		logrus.WithError(err).Warning("failed to determine latest released version of av")
		return
	}
	logrus.WithField("latest", latest).Debug("fetched latest released version")
	if semver.Compare(config.Version, latest) < 0 {
		c := color.New(color.Faint, color.Bold)
		_, _ = fmt.Fprint(
			os.Stderr,
			c.Sprint(">> A new version of av is available: "),
			color.RedString(config.Version),
			c.Sprint(" => "),
			color.GreenString(latest),
			"\n",
			c.Sprint(">> https://docs.aviator.co/reference/aviator-cli/installation#upgrade\n"),
		)
	}
}

var cachedRepo *git.Repo

func getRepo() (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		if rootFlags.Directory != "" {
			cmd.Dir = rootFlags.Directory
		}
		toplevel, err := cmd.Output()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine repo toplevel (are you running inside a Git repo?)")
		}
		cachedRepo, err = git.OpenRepo(strings.TrimSpace(string(toplevel)))
		if err != nil {
			return nil, errors.Wrap(err, "failed to open git repo")
		}
	}
	return cachedRepo, nil
}

var once sync.Once
var lazyGithubClient *gh.Client

func getClient(token string) (*gh.Client, error) {
	var err error
	once.Do(func() {
		lazyGithubClient, err = gh.NewClient(token)
	})
	return lazyGithubClient, err
}
