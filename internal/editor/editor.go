package editor

import (
	"bufio"
	"bytes"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

type Config struct {
	// The text to be edited.
	// After the editor is closed, the contents will be written back to this field.
	Text string
	// The file pattern to use when creating the temporary file for the editor.
	TmpFilePattern string
	// The prefix used to identify comments in the text.
	CommentPrefix string
	// The editor command to be used.
	// If empty, the git default editor will be used.
	Command string
}

func Launch(repo *git.Repo, config Config) (string, error) {
	switch {
	case config.Command == "":
		config.Command = DefaultCommand(repo)
	case config.TmpFilePattern == "":
		config.TmpFilePattern = "av-message-*"
	}

	tmp, err := os.CreateTemp("", config.TmpFilePattern)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil {
			logrus.WithError(err).Warn("failed to remove temporary file")
		}
	}()
	if _, err := tmp.WriteString(config.Text); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	// launch the editor as a subprocess
	cmd := exec.Command(config.Command, tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr
	logrus.WithField("cmd", cmd.String()).Debug("launching editor")
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"cmd": cmd.String(),
			"out": stderr.String(),
		}).Warn("editor exited with error")
		return "", err
	}

	return parseResult(tmp.Name(), config)
}

func DefaultCommand(repo *git.Repo) string {
	editor, err := repo.Git("var", "GIT_EDITOR")
	if err != nil {
		logrus.WithError(err).Warn("failed to determine desired editor from git config")
		// This is the default hard-coded into git
		return "vi"
	}
	return editor
}

func parseResult(path string, config Config) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	scan := bufio.NewScanner(f)
	res := bytes.NewBuffer(nil)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, config.CommentPrefix) {
			res.WriteString(line)
			res.WriteString("\n")
		}
	}
	return res.String(), nil
}
