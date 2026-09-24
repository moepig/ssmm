package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/awsconfig"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/session"
	"github.com/uncho/ssmm/internal/target"
	"github.com/uncho/ssmm/internal/terminal"
	"github.com/uncho/ssmm/internal/tui"
)

type Dependencies struct {
	Service        app.Service
	Store          app.SettingsStore
	Profiles       app.ProfileChecker
	Runner         app.Runner
	SSMMExecutable string
	AWSExecutable  string
	SSHExecutable  string
	SCPExecutable  string
	Input          *os.File
	Output         io.Writer
	Error          io.Writer
}

type CLI struct{ deps Dependencies }

func New(deps Dependencies) *cobra.Command {
	if deps.Input == nil {
		deps.Input = os.Stdin
	}
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	if deps.Error == nil {
		deps.Error = os.Stderr
	}
	c := &CLI{deps: deps}
	root := &cobra.Command{Use: "ssmm [TARGET]", Short: "connect to EC2 instances through AWS Systems Manager", SilenceUsage: true, SilenceErrors: true, Args: cobra.MaximumNArgs(1)}
	root.SetOut(deps.Output)
	root.SetErr(deps.Error)
	addSearchFlags(root)
	root.RunE = c.runConnect
	root.AddCommand(c.listCommand(), c.connectCommand(), c.sshCommand(), c.scpCommand(), c.proxyCommand(), c.initCommand(), c.sshConfigCommand())
	return root
}

func (c *CLI) ssmmProfile(cmd *cobra.Command) target.ProfileSelection {
	value, _ := cmd.Flags().GetString("ssmm-profile")
	if cmd.Flags().Changed("ssmm-profile") {
		return target.ProfileSelection{Name: value, Source: target.ProfileFlag}
	}
	if value == "default" {
		return target.ProfileSelection{Name: value, Source: target.ProfileDefault}
	}
	return target.ProfileSelection{}
}

func (c *CLI) awsProfile(cmd *cobra.Command) target.ProfileSelection {
	value, _ := cmd.Flags().GetString("profile")
	return awsconfig.SelectionFromFlags(value, cmd.Flags().Changed("profile"))
}

func (c *CLI) searchRequest(cmd *cobra.Command, targetName string) (app.SearchRequest, error) {
	region, _ := cmd.Flags().GetString("region")
	filter, _ := cmd.Flags().GetString("filter")
	tags, err := parseTags(cmd)
	if err != nil {
		return app.SearchRequest{}, err
	}
	return app.SearchRequest{Profile: c.ssmmProfile(cmd), AWSProfile: c.awsProfile(cmd), Target: stripUser(targetName), Region: region, Filter: filter, Tags: tags}, nil
}
func parseTags(cmd *cobra.Command) ([]target.TagFilter, error) {
	if cmd.Flags().Lookup("tag") == nil {
		return nil, nil
	}
	values, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		return nil, err
	}
	out := make([]target.TagFilter, 0, len(values))
	for _, value := range values {
		idx := strings.IndexByte(value, '=')
		if idx <= 0 || idx == len(value)-1 {
			return nil, fmt.Errorf("invalid tag %q; expected KEY=VALUE", value)
		}
		out = append(out, target.TagFilter{Key: value[:idx], Value: value[idx+1:]})
	}
	return out, nil
}

func (c *CLI) listCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{Use: "list", Short: "list EC2 instances", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		region, _ := cmd.Flags().GetString("region")
		filter, _ := cmd.Flags().GetString("filter")
		tags, err := parseTags(cmd)
		if err != nil {
			return exitError(2, "%v", err)
		}
		settings, scope, snapshot, _, err := c.deps.Service.Inventory(cmd.Context(), app.SearchRequest{Profile: c.ssmmProfile(cmd), AWSProfile: c.awsProfile(cmd), Region: region, Filter: filter, Tags: tags})
		if err != nil {
			return err
		}
		if !snapshot.EC2Complete {
			PrintDiagnostics(c.deps.Error, snapshot)
			return exitError(1, "EC2 inventory is incomplete")
		}
		if err := PrintList(c.deps.Output, filteredSnapshot(snapshot, filter), c.ssmmProfile(cmd).Name, scope, format); err != nil {
			return err
		}
		PrintDiagnostics(c.deps.Error, snapshot)
		_ = settings
		return nil
	}}
	cmd.Flags().StringVar(&format, "output", defaultListFormat, "output format: human-readable table (default), json, or id")
	addSearchFlags(cmd)
	return cmd
}

func filteredSnapshot(snapshot inventory.InventorySnapshot, filter string) inventory.InventorySnapshot {
	snapshot.Instances = target.Filter(snapshot.Instances, filter)
	return snapshot
}

func (c *CLI) connectCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "connect [TARGET]", Short: "connect to a target", Args: cobra.MaximumNArgs(1), RunE: c.runConnect}
	addSearchFlags(cmd)
	return cmd
}

func addSearchFlags(cmd *cobra.Command) {
	addConnectionFlags(cmd)
	cmd.Flags().String("filter", "", "case-insensitive partial-match filter")
	cmd.Flags().Bool("non-interactive", false, "disable target selection")
}

func addConnectionFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("ssmm-profile", "s", "", "optional ssmm profile")
	cmd.Flags().StringP("profile", "p", "", "AWS profile (overrides ssmm settings)")
	cmd.Flags().StringP("region", "r", "", "limit search to one AWS region")
	cmd.Flags().StringArrayP("tag", "t", nil, "EC2 tag filter KEY=VALUE (repeatable)")
}

func (c *CLI) runConnect(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	result, err := c.searchAndChoose(cmd, name, nonInteractive)
	if err != nil {
		return err
	}
	resolved := result.Target
	if c.deps.Service.Session == nil || c.deps.Runner == nil {
		return fmt.Errorf("session dependencies are incomplete")
	}
	if err := session.CheckDependencies(); err != nil {
		return err
	}
	spec, err := c.deps.Service.Session.PlanSession(app.SessionRequest{Target: resolved})
	if err != nil {
		return err
	}
	processResult, err := c.deps.Runner.Run(cmd.Context(), spec, execplan.Streams{Stdin: c.deps.Input, Stdout: c.outputFile(), Stderr: c.errorFile()})
	if err != nil {
		return err
	}
	return processExit(processResult)
}

func (c *CLI) runInventory(cmd *cobra.Command, name string, nonInteractive bool) (app.SearchResult, error) {
	req, err := c.searchRequest(cmd, name)
	if err != nil {
		return app.SearchResult{}, exitError(2, "%v", err)
	}
	settings, scope, snapshot, q, err := c.deps.Service.Inventory(cmd.Context(), req)
	if err != nil {
		return app.SearchResult{}, err
	}
	return app.SearchResult{Profile: req.Profile, AWSProfile: settings.AWSSelection(req.Profile.Name, req.AWSProfile), Settings: settings, Scope: scope, Snapshot: snapshot, Query: q, Filter: req.Filter}, nil
}

func (c *CLI) searchAndChoose(cmd *cobra.Command, name string, nonInteractive bool) (app.SearchResult, error) {
	if name != "" {
		nonInteractive = true
	}
	if nonInteractive || !terminal.Available() {
		result, err := c.runInventory(cmd, name, nonInteractive)
		if err != nil {
			return app.SearchResult{}, err
		}
		resolved, err := c.choose(cmd.Context(), result, name, true)
		if err != nil {
			return app.SearchResult{}, err
		}
		result.Target = resolved
		return result, nil
	}
	req, err := c.searchRequest(cmd, name)
	if err != nil {
		return app.SearchResult{}, exitError(2, "%v", err)
	}
	return c.interactiveSearch(cmd, req)
}

func (c *CLI) interactiveSearch(cmd *cobra.Command, req app.SearchRequest) (app.SearchResult, error) {
	filter := req.Filter
	var selected *target.InstanceKey
	for generation := uint64(1); ; generation++ {
		search, err := c.deps.Service.StartInventory(cmd.Context(), req, generation)
		if err != nil {
			return app.SearchResult{}, err
		}
		tty, err := terminal.Open()
		if err != nil {
			search.Run.Cancel()
			<-search.Run.Done
			return app.SearchResult{}, err
		}
		selection, selectErr := tui.SelectLive(cmd.Context(), tty.File, tty.File, search.Run.Snapshots, filter, selected)
		_ = tty.Close()
		search.Run.Cancel()
		<-search.Run.Done
		if selectErr != nil {
			return app.SearchResult{}, selectErr
		}
		if selection.Refresh {
			filter = selection.Filter
			if selection.HasKey {
				key := selection.Key
				selected = &key
			} else {
				selected = nil
			}
			continue
		}
		if selection.Canceled {
			return app.SearchResult{}, exitError(130, "selection canceled")
		}
		if selection.Generation != search.Generation {
			return app.SearchResult{}, fmt.Errorf("stale selection from search generation %d", selection.Generation)
		}
		if !selection.HasKey {
			return app.SearchResult{}, fmt.Errorf("no instance selected")
		}
		key := selection.Key
		resolved, err := app.ResolveSnapshot(search.AWSProfile, selection.Snapshot, search.Query, selection.Filter, &key, target.ResolvedManual)
		if err != nil {
			return app.SearchResult{}, err
		}
		return app.SearchResult{Profile: search.Profile, AWSProfile: search.AWSProfile, Settings: search.Settings, Scope: search.Scope, Snapshot: selection.Snapshot, Target: resolved, Query: search.Query, Filter: selection.Filter}, nil
	}
}

func (c *CLI) choose(ctx context.Context, result app.SearchResult, name string, nonInteractive bool) (target.ResolvedTarget, error) {
	filter := result.Filter
	query := result.Query
	profile := result.AWSProfile
	if name != "" && result.Snapshot.Finished && result.Snapshot.EC2Complete && len(app.Candidates(result.Snapshot, query, filter)) == 1 {
		resolved, err := app.ResolveSnapshot(profile, result.Snapshot, query, filter, nil, target.ResolvedUnique)
		if err != nil {
			return target.ResolvedTarget{}, err
		}
		return resolved, nil
	}
	if nonInteractive || !terminal.Available() {
		resolved, err := app.ResolveSnapshot(profile, result.Snapshot, query, filter, nil, target.ResolvedUnique)
		if err != nil {
			return target.ResolvedTarget{}, err
		}
		return resolved, nil
	}
	tty, err := terminal.Open()
	if err != nil {
		return target.ResolvedTarget{}, err
	}
	defer tty.Close()
	selection, err := tui.Select(ctx, tty.File, tty.File, result.Snapshot, filter)
	if err != nil {
		return target.ResolvedTarget{}, err
	}
	if selection.Canceled {
		return target.ResolvedTarget{}, exitError(130, "selection canceled")
	}
	return app.ResolveSnapshot(profile, result.Snapshot, query, selection.Filter, &selection.Key, target.ResolvedManual)
}

func stripUser(value string) string {
	if idx := strings.LastIndexByte(value, '@'); idx >= 0 {
		return value[idx+1:]
	}
	return value
}

func (c *CLI) outputFile() *os.File {
	if c.deps.Output == os.Stdout {
		return os.Stdout
	}
	return os.Stdout
}
func (c *CLI) errorFile() *os.File {
	if c.deps.Error == os.Stderr {
		return os.Stderr
	}
	return os.Stderr
}

func processExit(result execplan.ProcessResult) error {
	if result.ExitCode != 0 {
		return &ExitError{Code: result.ExitCode, Err: fmt.Errorf("external command exited with code %d", result.ExitCode)}
	}
	return nil
}
