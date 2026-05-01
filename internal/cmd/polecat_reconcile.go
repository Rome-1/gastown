package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/polecat"
	"github.com/steveyegge/gastown/internal/rig"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/tmux"
)

// gt-j3hj/C: 'gt polecat reconcile' command for manual / one-shot state repair.
//
// Detects polecats whose agent_state is stuck in an active value (working,
// running, spawning, done) when the evidence shows their work completed
// (hook bead closed, session dead, or witness-notified checkpoint set).
// Resets agent_state to idle and clears lingering done-cp:* / done-intent:*
// labels. Idempotent: safe to run repeatedly.

// ReconcileVerdict is the outcome of evaluating a single polecat.
type ReconcileVerdict string

const (
	ReconcileSkip       ReconcileVerdict = "skip"
	ReconcileWouldFix   ReconcileVerdict = "would-fix"
	ReconcileReconciled ReconcileVerdict = "reconciled"
	ReconcileFailed     ReconcileVerdict = "failed"
)

// ReconcileResult describes the reconcile outcome for one polecat.
type ReconcileResult struct {
	Rig          string           `json:"rig"`
	Polecat      string           `json:"polecat"`
	PriorState   string           `json:"prior_state"`
	NewState     string           `json:"new_state,omitempty"`
	Verdict      ReconcileVerdict `json:"verdict"`
	Reason       string           `json:"reason"`
	HookBead     string           `json:"hook_bead,omitempty"`
	HookStatus   string           `json:"hook_status,omitempty"`
	SessionAlive bool             `json:"session_alive"`
	Forced       bool             `json:"forced,omitempty"`
	Error        string           `json:"error,omitempty"`
}

var (
	polecatReconcileRig     string
	polecatReconcilePolecat string
	polecatReconcileDryRun  bool
	polecatReconcileForce   bool
	polecatReconcileJSON    bool
)

var polecatReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile polecats stuck in working/running with closed hook bead",
	Long: `Reconcile polecats whose agent_state is stuck in working/running/spawning/done
when the evidence shows their work completed (hook bead closed, session dead, or
the gt done witness-notified checkpoint is set).

Symptoms this fixes:
  - Polecat ran 'gt done' successfully (hook bead closed, MR submitted) but
    agent_state is still 'working' — typically caused by a Dolt outage during
    the final state write in updateAgentStateOnDone (see gt-j3hj/A).
  - Polecat slot occupied by a "stuck-in-working" polecat that no longer has
    a live session, blocking re-use of the slot for new work.

Reconcile rules:
  - state in {working, running, spawning, done} AND hook bead closed/empty
    AND session dead          -> reconcile to idle (clear done-cp/done-intent)
  - state in {working, running} AND done-cp:witness-notified label set
                              -> reconcile to idle (gt done passed witness notify)
  - --force: also reconcile when hook closed AND session dead even without
    a witness-notified checkpoint
  - Anything else             -> skip with reason

Examples:
  gt polecat reconcile                                # all polecats in all rigs
  gt polecat reconcile --rig gastown                  # all polecats in one rig
  gt polecat reconcile --polecat gastown/furiosa      # one polecat
  gt polecat reconcile --dry-run                      # preview without writes
  gt polecat reconcile --json                         # machine-readable output
  gt polecat reconcile --force --dry-run              # see what --force would do
`,
	RunE: runPolecatReconcile,
}

func init() {
	polecatReconcileCmd.Flags().StringVar(&polecatReconcileRig, "rig", "", "Limit to a specific rig")
	polecatReconcileCmd.Flags().StringVar(&polecatReconcilePolecat, "polecat", "", "Reconcile a single polecat (rig/name)")
	polecatReconcileCmd.Flags().BoolVar(&polecatReconcileDryRun, "dry-run", false, "Show what would change without writing")
	polecatReconcileCmd.Flags().BoolVar(&polecatReconcileForce, "force", false, "Reconcile even without witness-notified checkpoint when hook closed + session dead")
	polecatReconcileCmd.Flags().BoolVar(&polecatReconcileJSON, "json", false, "Output as JSON")

	polecatCmd.AddCommand(polecatReconcileCmd)
}

func runPolecatReconcile(cmd *cobra.Command, args []string) error {
	rigs, singlePolecat, err := resolveReconcileScope()
	if err != nil {
		return err
	}

	t := tmux.NewTmux()
	var results []ReconcileResult

	for _, r := range rigs {
		bd := beads.New(r.Path)
		polecatMgr := polecat.NewSessionManager(t, r)

		var names []string
		if singlePolecat != "" {
			names = []string{singlePolecat}
		} else {
			polecats, listErr := listRigPolecatNames(r)
			if listErr != nil {
				fmt.Fprintf(os.Stderr, "warning: listing polecats in %s: %v\n", r.Name, listErr)
				continue
			}
			names = polecats
		}

		for _, name := range names {
			res := evaluatePolecatReconcile(bd, polecatMgr, r, name)
			if res.Verdict == ReconcileWouldFix && !polecatReconcileDryRun {
				res = applyPolecatReconcile(bd, r, name, res)
			}
			results = append(results, res)
		}
	}

	return outputReconcileResults(results)
}

// resolveReconcileScope determines which rigs and polecat to operate on
// based on the flag combination. Returns (rigs, singlePolecatName, err).
func resolveReconcileScope() ([]*rig.Rig, string, error) {
	if polecatReconcilePolecat != "" {
		rigName, polecatName, err := parseAddress(polecatReconcilePolecat)
		if err != nil {
			return nil, "", fmt.Errorf("invalid --polecat address: %w", err)
		}
		_, r, err := getRig(rigName)
		if err != nil {
			return nil, "", err
		}
		return []*rig.Rig{r}, polecatName, nil
	}

	if polecatReconcileRig != "" {
		_, r, err := getRig(polecatReconcileRig)
		if err != nil {
			return nil, "", err
		}
		return []*rig.Rig{r}, "", nil
	}

	allRigs, err := getAllRigs()
	if err != nil {
		return nil, "", err
	}
	return allRigs, "", nil
}

// listRigPolecatNames returns polecat names for a rig.
func listRigPolecatNames(r *rig.Rig) ([]string, error) {
	mgr, _, err := getPolecatManager(r.Name)
	if err != nil {
		return nil, err
	}
	polecats, err := mgr.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(polecats))
	for _, p := range polecats {
		names = append(names, p.Name)
	}
	return names, nil
}

// evaluatePolecatReconcile inspects one polecat and returns the verdict
// without making any writes.
func evaluatePolecatReconcile(bd *beads.Beads, polecatMgr *polecat.SessionManager, r *rig.Rig, polecatName string) ReconcileResult {
	res := ReconcileResult{
		Rig:     r.Name,
		Polecat: polecatName,
	}

	agentBeadID := polecatBeadIDForRig(r, r.Name, polecatName)
	issue, fields, err := bd.GetAgentBead(agentBeadID)
	if err != nil || issue == nil {
		res.Verdict = ReconcileSkip
		res.Reason = fmt.Sprintf("no agent bead (%v)", err)
		return res
	}

	res.PriorState = issue.AgentState

	hookBead := issue.HookBead
	if hookBead == "" && fields != nil {
		hookBead = fields.HookBead
	}
	res.HookBead = hookBead

	if hookBead != "" {
		hookIssue, hookErr := bd.Show(hookBead)
		if hookErr == nil && hookIssue != nil {
			res.HookStatus = hookIssue.Status
		}
	}

	if polecatMgr != nil {
		alive, _ := polecatMgr.IsRunning(polecatName)
		res.SessionAlive = alive
	}

	if !shouldReconcileState(res.PriorState) {
		res.Verdict = ReconcileSkip
		res.Reason = fmt.Sprintf("state already terminal/safe: %s", res.PriorState)
		return res
	}

	hasWitnessCheckpoint := hasLabelPrefix(issue.Labels, "done-cp:witness-notified:")
	hookClosedOrEmpty := hookBead == "" || res.HookStatus == "closed"

	switch {
	case hookClosedOrEmpty && !res.SessionAlive:
		res.Verdict = ReconcileWouldFix
		res.NewState = string(beads.AgentStateIdle)
		res.Reason = fmt.Sprintf("hook %s, session dead", hookStatusLabel(hookBead, res.HookStatus))
	case hasWitnessCheckpoint && (beads.AgentState(res.PriorState) == beads.AgentStateWorking || beads.AgentState(res.PriorState) == beads.AgentStateRunning):
		res.Verdict = ReconcileWouldFix
		res.NewState = string(beads.AgentStateIdle)
		res.Reason = "done-cp:witness-notified set (gt done past witness notify)"
	case polecatReconcileForce && hookClosedOrEmpty && !res.SessionAlive:
		// Same as first case — defensive.
		res.Verdict = ReconcileWouldFix
		res.NewState = string(beads.AgentStateIdle)
		res.Reason = "forced: hook closed/empty, session dead"
		res.Forced = true
	default:
		res.Verdict = ReconcileSkip
		switch {
		case res.SessionAlive:
			res.Reason = "session alive"
		case hookBead != "" && res.HookStatus != "closed":
			res.Reason = fmt.Sprintf("hook bead %s status=%s (not closed)", hookBead, res.HookStatus)
		default:
			res.Reason = "no reconcile signal"
		}
	}

	return res
}

// applyPolecatReconcile performs the reconcile writes for a single polecat
// and returns an updated result with the verdict reflecting success/failure.
func applyPolecatReconcile(bd *beads.Beads, r *rig.Rig, polecatName string, res ReconcileResult) ReconcileResult {
	agentBeadID := polecatBeadIDForRig(r, r.Name, polecatName)

	var stateErr error
	for attempt := 1; attempt <= 3; attempt++ {
		_, stateErr = bd.Run("agent", "state", agentBeadID, res.NewState)
		if stateErr == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}
	if stateErr != nil {
		res.Verdict = ReconcileFailed
		res.Error = stateErr.Error()
		return res
	}

	// Best-effort label cleanup. Re-read labels after the state write to get
	// the latest snapshot (avoids racing with concurrent writers).
	if issue, err := bd.Show(agentBeadID); err == nil && issue != nil {
		var toRemove []string
		for _, label := range issue.Labels {
			if strings.HasPrefix(label, "done-cp:") || strings.HasPrefix(label, "done-intent:") {
				toRemove = append(toRemove, label)
			}
		}
		if len(toRemove) > 0 {
			if err := bd.Update(agentBeadID, beads.UpdateOptions{RemoveLabels: toRemove}); err != nil {
				fmt.Fprintf(os.Stderr, "warning: clearing checkpoint labels on %s: %v\n", agentBeadID, err)
			}
		}
	}

	res.Verdict = ReconcileReconciled
	return res
}

func shouldReconcileState(state string) bool {
	switch beads.AgentState(state) {
	case beads.AgentStateWorking, beads.AgentStateRunning,
		beads.AgentStateSpawning, beads.AgentStateDone:
		return true
	default:
		return false
	}
}

func hasLabelPrefix(labels []string, prefix string) bool {
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			return true
		}
	}
	return false
}

func hookStatusLabel(hookBead, status string) string {
	if hookBead == "" {
		return "empty"
	}
	if status == "" {
		return fmt.Sprintf("closed/unknown (%s)", hookBead)
	}
	return fmt.Sprintf("%s (%s)", status, hookBead)
}

func outputReconcileResults(results []ReconcileResult) error {
	if polecatReconcileJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(results) == 0 {
		fmt.Println("No polecats found to evaluate.")
		return nil
	}

	var fixed, would, skipped, failed int
	fmt.Printf("%s\n\n", style.Bold.Render("Polecat Reconcile"))
	fmt.Printf("  %-30s %-10s %-12s %-12s %s\n",
		"POLECAT", "PRIOR", "ACTION", "VERDICT", "REASON")
	fmt.Printf("  %s\n", strings.Repeat("-", 90))
	for _, r := range results {
		address := fmt.Sprintf("%s/%s", r.Rig, r.Polecat)
		action := "-"
		if r.NewState != "" {
			action = fmt.Sprintf("→ %s", r.NewState)
		}

		verdict := string(r.Verdict)
		switch r.Verdict {
		case ReconcileReconciled:
			verdict = style.Success.Render(verdict)
			fixed++
		case ReconcileWouldFix:
			verdict = style.Info.Render(verdict)
			would++
		case ReconcileSkip:
			verdict = style.Dim.Render(verdict)
			skipped++
		case ReconcileFailed:
			verdict = style.Error.Render(verdict)
			failed++
		}

		fmt.Printf("  %-30s %-10s %-12s %-22s %s\n",
			address, r.PriorState, action, verdict, r.Reason)
		if r.Error != "" {
			fmt.Printf("    %s %s\n", style.Error.Render("error:"), r.Error)
		}
	}

	fmt.Println()
	if polecatReconcileDryRun {
		fmt.Printf("%s %d would-fix, %d skipped (dry-run, no writes)\n",
			style.Info.Render("→"), would, skipped)
	} else {
		fmt.Printf("%s %d reconciled, %d skipped, %d failed\n",
			style.Info.Render("→"), fixed, skipped, failed)
	}

	if failed > 0 {
		return fmt.Errorf("%d reconcile(s) failed", failed)
	}
	return nil
}
