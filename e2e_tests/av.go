package e2e_tests

import (
	"bytes"
	"emperror.dev/errors"
	"fmt"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var avCmdPath string

func init() {
	if err := os.Setenv("AV_GITHUB_TOKEN", "ghp_thisisntarealltokenitsjustfortesting"); err != nil {
		logrus.WithError(err).Fatal("failed to set AV_GITHUB_TOKEN env var")
	}

	logrus.SetLevel(logrus.DebugLevel)
	cmd := exec.Command("go", "build", "../cmd/av")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}
	var err error
	avCmdPath, err = filepath.Abs("./av")
	if err != nil {
		panic(err)
	}
}

type AvOutput struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func Cmd(t *testing.T, exe string, args ...string) AvOutput {
	cmd := exec.Command(exe, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatal(err)
	}

	output := AvOutput{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	fmt.Printf("\nRunning command:\n    %v\n", args)
	fmt.Printf("    exit code: %v\n", cmd.ProcessState.ExitCode())
	fmt.Printf("    stdout:\n%s\n", text.Indent(stdout.String(), "        "))
	fmt.Printf("    stderr:\n%s\n", text.Indent(stderr.String(), "        "))
	fmt.Printf("\n")

	return output
}

func RequireCmd(t *testing.T, exe string, args ...string) AvOutput {
	output := Cmd(t, exe, args...)
	require.Equal(t, 0, output.ExitCode, "cmd %s: exited with %v", args, output.ExitCode)
	return output
}

func Av(t *testing.T, args ...string) AvOutput {
	args = append([]string{"--debug"}, args...)
	return Cmd(t, avCmdPath, args...)
}

func RequireAv(t *testing.T, args ...string) AvOutput {
	output := Av(t, args...)
	require.Equal(t, 0, output.ExitCode, "av %s: exited with %v", args, output.ExitCode)
	return output
}

func Chdir(t *testing.T, dir string) {
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(current); err != nil {
			t.Fatal(err)
		}
	})
}
