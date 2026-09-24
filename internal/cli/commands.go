package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/config"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/openssh"
	"github.com/uncho/ssmm/internal/proxy"
	"github.com/uncho/ssmm/internal/session"
	"github.com/uncho/ssmm/internal/target"
	"github.com/uncho/ssmm/internal/terminal"
)

func (c *CLI) sshCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "ssh [USER@]TARGET", Short: "connect using OpenSSH (experimental)", Args: cobra.MaximumNArgs(1), RunE: c.runSSH}
	addSearchFlags(cmd)
	cmd.Flags().String("user", "", "SSH user")
	cmd.Flags().String("identity-file", "", "SSH identity file")
	cmd.Flags().Int("port", 0, "SSH port")
	return cmd
}

func (c *CLI) scpCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "scp [OPTIONS] SRC... DEST", Short: "copy files using OpenSSH (experimental)", Args: cobra.MinimumNArgs(2), RunE: c.runSCP}
	addConnectionFlags(cmd)
	cmd.Flags().String("user", "", "SSH user")
	cmd.Flags().String("identity-file", "", "SSH identity file")
	cmd.Flags().Int("port", 0, "SSH port")
	cmd.Flags().Bool("recursive", false, "copy directories recursively")
	cmd.Flags().Bool("non-interactive", false, "disable target selection")
	return cmd
}

func (c *CLI) proxyCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "proxy TARGET", Short: "run as an OpenSSH ProxyCommand", Args: cobra.ExactArgs(1), RunE: c.runProxy}
	addConnectionFlags(cmd)
	cmd.Flags().Int("port", 22, "SSH port")
	cmd.Flags().String("internal-profile-source", "", "internal profile source")
	_ = cmd.Flags().MarkHidden("internal-profile-source")
	return cmd
}

func (c *CLI) initCommand() *cobra.Command {
	var regions []string
	var allRegions bool
	cmd := &cobra.Command{Use: "init [SSMM_PROFILE]", Short: "create or update ssmm settings", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ssh, _ := cmd.Flags().GetBool("ssh")
		if !ssh && (cmd.Flags().Changed("user") || cmd.Flags().Changed("identity-file") || cmd.Flags().Changed("port")) {
			return exitError(2, "--user, --identity-file, and --port require --ssh")
		}
		name := "default"
		if len(args) == 1 {
			name = args[0]
		}
		if cmd.Flags().Changed("ssmm-profile") {
			flagName, _ := cmd.Flags().GetString("ssmm-profile")
			if len(args) == 1 && flagName != name {
				return exitError(2, "positional ssmm profile %q conflicts with --ssmm-profile %q", name, flagName)
			}
			name = flagName
		}
		if c.deps.Store == nil {
			return fmt.Errorf("settings store is unavailable")
		}
		if err := target.ValidateSSMMProfileName(name); err != nil {
			return exitError(2, "%v", err)
		}
		draft, err := c.deps.Store.ReadForUpdate(cmd.Context(), false)
		if err != nil {
			return err
		}
		profile := draft.Snapshot.Profiles[name]
		if !cmd.Flags().Changed("regions") && !cmd.Flags().Changed("all-regions") && !cmd.Flags().Changed("user") && !cmd.Flags().Changed("identity-file") && !cmd.Flags().Changed("port") && !cmd.Flags().Changed("profile") && !cmd.Flags().Changed("region") && terminal.Available() {
			if err := c.editInitInteractively(&profile, ssh); err != nil {
				return err
			}
		}
		if cmd.Flags().Changed("profile") {
			profile.AWSProfile, _ = cmd.Flags().GetString("profile")
		}
		if cmd.Flags().Changed("region") {
			region, _ := cmd.Flags().GetString("region")
			profile.Regions = &[]string{region}
		}
		if cmd.Flags().Changed("all-regions") {
			if allRegions && cmd.Flags().Changed("regions") {
				return exitError(2, "--all-regions conflicts with --regions")
			}
			profile.Regions = nil
		}
		if cmd.Flags().Changed("regions") {
			if len(regions) == 0 {
				return exitError(2, "regions cannot be empty")
			}
			profile.Regions = &regions
		}
		if cmd.Flags().Changed("user") {
			profile.SSH.User, _ = cmd.Flags().GetString("user")
		}
		if cmd.Flags().Changed("identity-file") {
			value, _ := cmd.Flags().GetString("identity-file")
			cwd, _ := os.Getwd()
			profile.SSH.IdentityFile, err = config.ExpandFlagPath(value, cwd)
			if err != nil {
				return exitError(2, "%v", err)
			}
			// Store the resolved value. A relative flag is based on the
			// invocation directory, while a later config reload is based on
			// the config directory.
			profile.StoredIdentityFile = profile.SSH.IdentityFile
		}
		if cmd.Flags().Changed("port") {
			profile.SSH.Port, _ = cmd.Flags().GetInt("port")
		}
		if draft.Snapshot.Profiles == nil {
			draft.Snapshot.Profiles = map[string]app.ProfileSettings{}
		}
		draft.Snapshot.Profiles[name] = profile
		return c.commitSettings(cmd, draft)
	}}
	cmd.Flags().StringP("ssmm-profile", "s", "default", "ssmm profile to create or update")
	cmd.Flags().StringP("profile", "p", "", "AWS profile to save")
	cmd.Flags().StringP("region", "r", "", "single search region to save")
	cmd.Flags().Bool("ssh", false, "configure SSH user, identity file, and port")
	cmd.Flags().StringSliceVar(&regions, "regions", nil, "search regions")
	cmd.Flags().BoolVar(&allRegions, "all-regions", false, "search all enabled regions")
	cmd.MarkFlagsMutuallyExclusive("region", "regions", "all-regions")
	cmd.Flags().String("user", "", "default SSH user (requires --ssh)")
	cmd.Flags().String("identity-file", "", "default SSH identity file (requires --ssh)")
	cmd.Flags().Int("port", 0, "default SSH port (requires --ssh)")
	return cmd
}

func (c *CLI) editInitInteractively(profile *app.ProfileSettings, ssh bool) error {
	tty, err := terminal.Open()
	if err != nil {
		return err
	}
	defer tty.Close()
	reader := bufio.NewReader(tty.File)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(signals)
	read := func(label, current string) (string, error) {
		fmt.Fprintf(tty.File, "%s [%s]: ", label, current)
		result := make(chan struct {
			value string
			err   error
		}, 1)
		go func() {
			value, err := reader.ReadString('\n')
			result <- struct {
				value string
				err   error
			}{value: value, err: err}
		}()
		var value string
		select {
		case <-signals:
			return "", exitError(130, "initialization canceled")
		case read := <-result:
			if read.err != nil {
				return "", exitError(130, "initialization canceled")
			}
			value = read.value
		}
		value = strings.TrimSpace(strings.TrimSuffix(value, "\n"))
		if strings.EqualFold(value, "cancel") || value == "\x1b" {
			return "", exitError(130, "initialization canceled")
		}
		return value, nil
	}

	value, err := read("AWS profile (empty = environment/default)", profile.AWSProfile)
	if err != nil {
		return err
	}
	if value != "" {
		profile.AWSProfile = value
	}

	currentRegions := ""
	if profile.Regions != nil {
		currentRegions = strings.Join(*profile.Regions, ",")
	}
	value, err = read("Regions (comma separated, empty = all)", currentRegions)
	if err != nil {
		return err
	}
	if value == "" {
		profile.Regions = nil
	} else {
		parts := strings.Split(value, ",")
		regions := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if !config.IsCommercialRegion(part) {
				return exitError(2, "invalid region %q", part)
			}
			regions = append(regions, part)
		}
		profile.Regions = &regions
	}
	if !ssh {
		return nil
	}
	if value, err = read("SSH user", profile.SSH.User); err != nil {
		return err
	} else if value != "" {
		profile.SSH.User = value
	}
	identity := profile.SSH.IdentityFile
	if identity == "" {
		identity = profile.StoredIdentityFile
	}
	if value, err = read("SSH identity file", identity); err != nil {
		return err
	} else if value != "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return cwdErr
		}
		profile.SSH.IdentityFile, err = config.ExpandFlagPath(value, cwd)
		if err != nil {
			return exitError(2, "%v", err)
		}
		profile.StoredIdentityFile = profile.SSH.IdentityFile
	}
	port := ""
	if profile.SSH.Port != 0 {
		port = strconv.Itoa(profile.SSH.Port)
	}
	if value, err = read("SSH port (empty = default)", port); err != nil {
		return err
	} else if value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return exitError(2, "invalid SSH port %q", value)
		}
		profile.SSH.Port = parsed
	}
	return nil
}

func (c *CLI) runSSH(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	user, err := operandUser(name)
	if err != nil {
		return exitError(2, "%v", err)
	}
	flagUser, _ := cmd.Flags().GetString("user")
	if flagUser != "" {
		if user != "" && user != flagUser {
			return exitError(2, "USER@TARGET conflicts with --user")
		}
		user = flagUser
	}
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	result, err := c.searchAndChoose(cmd, stripUser(name), nonInteractive)
	if err != nil {
		return err
	}
	resolved := result.Target
	profile := result.Settings.Profile(result.Profile.Name)
	options, err := c.sshOptions(cmd, userOr(user, profile.SSH.User), profile)
	if err != nil {
		return exitError(2, "%v", err)
	}
	if err := session.CheckDependencies(); err != nil {
		return err
	}
	if c.deps.Service.SSH == nil || c.deps.Runner == nil {
		return fmt.Errorf("SSH dependencies are incomplete")
	}
	spec, err := c.deps.Service.SSH.PlanSSH(app.SSHRequest{Target: resolved, Options: options, Executable: c.deps.SSHExecutable, SsmmExecutable: c.deps.SSMMExecutable})
	if err != nil {
		return err
	}
	return c.runSpec(cmd.Context(), spec)
}

func (c *CLI) runSCP(cmd *cobra.Command, args []string) error {
	transfer, err := openssh.ParseTransfer(args)
	if err != nil {
		return exitError(2, "%v", err)
	}
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	result, err := c.searchAndChoose(cmd, transfer.Target, nonInteractive)
	if err != nil {
		return err
	}
	resolved := result.Target
	profile := result.Settings.Profile(result.Profile.Name)
	flagUser, _ := cmd.Flags().GetString("user")
	user := flagUser
	if transfer.User != "" {
		if flagUser != "" && flagUser != transfer.User {
			return exitError(2, "USER@TARGET conflicts with --user")
		}
		user = transfer.User
	}
	options, err := c.sshOptions(cmd, userOr(user, profile.SSH.User), profile)
	if err != nil {
		return exitError(2, "%v", err)
	}
	recursive, _ := cmd.Flags().GetBool("recursive")
	if err := session.CheckDependencies(); err != nil {
		return err
	}
	if c.deps.Service.SSH == nil || c.deps.Runner == nil {
		return fmt.Errorf("SCP dependencies are incomplete")
	}
	spec, err := c.deps.Service.SSH.PlanSCP(app.SCPRequest{Target: resolved, Options: options, Executable: c.deps.SCPExecutable, SsmmExecutable: c.deps.SSMMExecutable, Transfer: transfer, Recursive: recursive})
	if err != nil {
		return err
	}
	return c.runSpec(cmd.Context(), spec)
}

func (c *CLI) runProxy(cmd *cobra.Command, args []string) error {
	value := args[0]
	profileName, _ := cmd.Flags().GetString("profile")
	ssmmName, _ := cmd.Flags().GetString("ssmm-profile")
	ssmmSet := cmd.Flags().Changed("ssmm-profile")
	region, _ := cmd.Flags().GetString("region")
	port, _ := cmd.Flags().GetInt("port")
	source, _ := cmd.Flags().GetString("internal-profile-source")
	if source != "" {
		req, err := proxy.ParseInternal([]string{value, "--region", region, "--port", fmt.Sprint(port), "--profile", profileName, "--internal-profile-source", source}, os.Getenv("AWS_PROFILE"))
		if err != nil {
			return exitError(2, "%v", err)
		}
		selection := target.ProfileSelection{Name: req.Profile, Source: target.ProfileSource(req.Source)}
		if err := c.deps.Profiles.Validate(cmd.Context(), selection); err != nil {
			return err
		}
		return c.runProxyTarget(cmd, target.ResolvedTarget{Profile: selection, Region: req.Region, InstanceID: req.InstanceID, Method: target.ResolvedDirect}, port)
	}
	if host, err := proxy.ParseHost(value); err == nil {
		if ssmmSet && ssmmName != host.Profile {
			return exitError(2, "host profile %q conflicts with --ssmm-profile %q", host.Profile, ssmmName)
		}
		return c.proxySearch(cmd, target.ProfileSelection{Name: host.Profile, Source: target.ProfileHost}, host.Target, region, port)
	}
	if strings.HasSuffix(value, ".ssmm") {
		return exitError(2, "invalid ssmm host %q", value)
	}
	selection := c.ssmmProfile(cmd)
	tags, err := parseTags(cmd)
	if err != nil {
		return exitError(2, "%v", err)
	}
	if _, err := target.ParseQuery(value, tags); err != nil {
		return exitError(2, "%v", err)
	}
	if target.IsInstanceID(value) && region != "" {
		settings, err := c.deps.Service.ReadSettings(cmd.Context(), selection)
		if err != nil {
			return err
		}
		selection = settings.AWSSelection(selection.Name, c.awsProfile(cmd))
		return c.runProxyTarget(cmd, target.ResolvedTarget{Profile: selection, Region: region, InstanceID: value, Method: target.ResolvedDirect}, port)
	}
	return c.proxySearch(cmd, selection, value, region, port)
}

func (c *CLI) proxySearch(cmd *cobra.Command, selection target.ProfileSelection, name, region string, port int) error {
	tags, err := parseTags(cmd)
	if err != nil {
		return exitError(2, "%v", err)
	}
	settings, scope, snapshot, query, err := c.deps.Service.Inventory(cmd.Context(), app.SearchRequest{Profile: selection, AWSProfile: c.awsProfile(cmd), Target: name, Region: region, Tags: tags})
	if err != nil {
		return err
	}
	_ = scope
	resolved, err := app.ResolveSnapshot(settings.AWSSelection(selection.Name, c.awsProfile(cmd)), snapshot, query, "", nil, target.ResolvedUnique)
	if err != nil {
		return err
	}
	return c.runProxyTarget(cmd, resolved, port)
}

func (c *CLI) runProxyTarget(cmd *cobra.Command, resolved target.ResolvedTarget, port int) error {
	if c.deps.Profiles != nil {
		if err := c.deps.Profiles.Validate(cmd.Context(), resolved.Profile); err != nil {
			return err
		}
	}
	if err := session.CheckDependencies(); err != nil {
		return err
	}
	if c.deps.Service.Session == nil || c.deps.Runner == nil {
		return fmt.Errorf("proxy dependencies are incomplete")
	}
	spec, err := c.deps.Service.Session.PlanSession(app.SessionRequest{Target: resolved, Proxy: true, Port: port})
	if err != nil {
		return err
	}
	return c.runSpec(cmd.Context(), spec)
}

func (c *CLI) runSpec(ctx context.Context, spec execplan.ProcessSpec) error {
	result, err := c.deps.Runner.Run(ctx, spec, execplan.Streams{Stdin: c.deps.Input, Stdout: c.outputFile(), Stderr: c.errorFile()})
	if err != nil {
		return err
	}
	return processExit(result)
}
func operandUser(value string) (string, error) {
	if idx := strings.LastIndexByte(value, '@'); idx >= 0 {
		if idx == 0 || idx == len(value)-1 {
			return "", fmt.Errorf("invalid USER@TARGET")
		}
		return value[:idx], nil
	}
	return "", nil
}
func userOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (c *CLI) sshOptions(cmd *cobra.Command, user string, profile app.ProfileSettings) (app.SSHOptions, error) {
	identity, err := cmd.Flags().GetString("identity-file")
	if err != nil {
		return app.SSHOptions{}, err
	}
	if identity != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return app.SSHOptions{}, err
		}
		identity, err = config.ExpandFlagPath(identity, cwd)
		if err != nil {
			return app.SSHOptions{}, err
		}
	} else {
		identity = profile.SSH.IdentityFile
	}
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return app.SSHOptions{}, err
	}
	if port == 0 {
		port = profile.SSH.Port
	}
	options := app.SSHOptions{User: user, IdentityFile: identity, Port: port}
	if err := options.Validate(); err != nil {
		return app.SSHOptions{}, err
	}
	return options, nil
}
