package commands

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/port-experimental/port-cli/internal/modules/skills"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// printLoadResult — output routing
// ---------------------------------------------------------------------------

func TestPrintLoadResult_WritesToStderr(t *testing.T) {
	result := &skills.LoadSkillsResult{
		RequiredCount: 1,
		SelectedCount: 3,
		TargetResults: []skills.TargetResult{
			{Path: "/home/user/.cursor", SkillCount: 4, IsProject: false},
		},
	}

	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW

	// Capture stdout to make sure nothing leaks there.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = stdoutW

	printLoadResult(result)

	stdoutW.Close()
	os.Stdout = origStdout
	stderrW.Close()
	os.Stderr = origStderr

	var stderrBuf, stdoutBuf bytes.Buffer
	io.Copy(&stderrBuf, stderrR)
	io.Copy(&stdoutBuf, stdoutR)

	if stderrBuf.Len() == 0 {
		t.Error("expected printLoadResult to write to stderr, but stderr was empty")
	}
	if stdoutBuf.Len() != 0 {
		t.Errorf("expected nothing on stdout, got: %q", stdoutBuf.String())
	}
}

func TestPrintLoadResult_IncludesSkillCount(t *testing.T) {
	result := &skills.LoadSkillsResult{
		RequiredCount: 2,
		SelectedCount: 5,
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w

	printLoadResult(result)

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("7")) {
		t.Errorf("expected total skill count (7) in output, got: %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("2")) {
		t.Errorf("expected required count (2) in output, got: %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("5")) {
		t.Errorf("expected selected count (5) in output, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// --quiet flag
// ---------------------------------------------------------------------------

func TestSkillsSync_QuietFlagRegistered(t *testing.T) {
	root := &cobra.Command{Use: "port"}
	RegisterSkills(root)

	syncCmd, _, err := root.Find([]string{"skills", "sync"})
	if err != nil || syncCmd == nil {
		t.Fatal("skills sync command not found")
	}

	if err := syncCmd.ParseFlags([]string{"--quiet"}); err != nil {
		t.Fatalf("failed to parse --quiet flag: %v", err)
	}

	quiet, err := syncCmd.Flags().GetBool("quiet")
	if err != nil {
		t.Fatalf("could not get --quiet flag: %v", err)
	}
	if !quiet {
		t.Error("expected --quiet to be true after parsing")
	}
}

func TestSkillsSync_QuietShorthandRegistered(t *testing.T) {
	root := &cobra.Command{Use: "port"}
	RegisterSkills(root)

	syncCmd, _, err := root.Find([]string{"skills", "sync"})
	if err != nil || syncCmd == nil {
		t.Fatal("skills sync command not found")
	}

	if err := syncCmd.ParseFlags([]string{"-q"}); err != nil {
		t.Fatalf("failed to parse -q flag: %v", err)
	}

	quiet, err := syncCmd.Flags().GetBool("quiet")
	if err != nil {
		t.Fatalf("could not get --quiet via -q: %v", err)
	}
	if !quiet {
		t.Error("expected --quiet to be true after parsing -q")
	}
}
