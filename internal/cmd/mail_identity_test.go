package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSenderFromCwdUsesAgentFileWitnessIdentity(t *testing.T) {
	t.Setenv("GT_ROLE", "")
	t.Setenv("GT_RIG", "")
	t.Setenv("GT_POLECAT", "")
	t.Setenv("GT_CREW", "")

	tmp := t.TempDir()
	witnessDir := filepath.Join(tmp, "x267", "witness")
	if err := os.MkdirAll(filepath.Join(witnessDir, "rig"), 0o755); err != nil {
		t.Fatalf("mkdir witness dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(witnessDir, ".gt-agent"),
		[]byte(`{"role":"witness","rig":"x267"}`),
		0o644,
	); err != nil {
		t.Fatalf("write .gt-agent: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(filepath.Join(witnessDir, "rig")); err != nil {
		t.Fatalf("chdir witness rig dir: %v", err)
	}

	got := detectSender()
	if got != "x267/witness" {
		t.Fatalf("detectSender() = %q, want %q", got, "x267/witness")
	}
}

func TestDetectSenderFromRoleUnknownUsesRoleDirectly(t *testing.T) {
	// Custom agent templates (e.g. "recurse") set GT_ROLE to their template name
	// but are not in the standard role switch. detectSender() should return the
	// role directly — it is a valid inbox address — rather than falling through
	// to cwd detection and producing "overseer".
	t.Setenv("GT_ROLE", "recurse")
	t.Setenv("GT_RIG", "")
	t.Setenv("GC_SESSION_NAME", "")

	got := detectSender()
	if got != "recurse" {
		t.Fatalf("detectSender() = %q, want %q", got, "recurse")
	}
}

func TestDetectSenderFromCwdFallsBackToSessionName(t *testing.T) {
	// When GT_ROLE is not set and cwd doesn't match any known agent path,
	// GC_SESSION_NAME should be used as the fallback identity — not "overseer".
	t.Setenv("GT_ROLE", "")
	t.Setenv("GT_RIG", "")
	t.Setenv("GC_SESSION_NAME", "recurse")

	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}

	got := detectSender()
	if got != "recurse" {
		t.Fatalf("detectSender() = %q, want %q", got, "recurse")
	}
}

func TestDetectSenderFromCwdUsesAgentFileRefineryIdentity(t *testing.T) {
	t.Setenv("GT_ROLE", "")
	t.Setenv("GT_RIG", "")
	t.Setenv("GT_POLECAT", "")
	t.Setenv("GT_CREW", "")

	tmp := t.TempDir()
	refineryDir := filepath.Join(tmp, "x267", "refinery")
	if err := os.MkdirAll(filepath.Join(refineryDir, "rig"), 0o755); err != nil {
		t.Fatalf("mkdir refinery dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(refineryDir, ".gt-agent"),
		[]byte(`{"role":"refinery","rig":"x267"}`),
		0o644,
	); err != nil {
		t.Fatalf("write .gt-agent: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(filepath.Join(refineryDir, "rig")); err != nil {
		t.Fatalf("chdir refinery rig dir: %v", err)
	}

	got := detectSender()
	if got != "x267/refinery" {
		t.Fatalf("detectSender() = %q, want %q", got, "x267/refinery")
	}
}
