package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uncho/ssmm/internal/app"
)

func (c *CLI) sshConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-config",
		Short: "manage SSH config for standard ssh and scp commands",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().StringP("ssmm-profile", "s", "default", "ssmm profile")
	cmd.AddCommand(
		c.sshConfigAction("create", "create or update SSH config for a profile", true),
		c.sshConfigAction("delete", "delete SSH config for a profile", false),
	)
	return cmd
}

func (c *CLI) sshConfigAction(name, short string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.deps.Store == nil {
				return fmt.Errorf("settings store is unavailable")
			}
			draft, err := c.deps.Store.ReadForUpdate(cmd.Context(), true)
			if err != nil {
				return err
			}
			name := c.ssmmProfile(cmd).Name
			profile := draft.Snapshot.Profile(name)
			profile.Integration = enabled
			if draft.Snapshot.Profiles == nil {
				draft.Snapshot.Profiles = map[string]app.ProfileSettings{}
			}
			draft.Snapshot.Profiles[name] = profile
			return c.commitSettings(cmd, draft)
		},
	}
}

func (c *CLI) commitSettings(cmd *cobra.Command, draft app.SettingsDraft) error {
	report, err := c.deps.Store.Commit(cmd.Context(), app.SettingsUpdate{UpdateSSH: draft.UpdateSSH, Snapshot: draft.Snapshot, Original: draft.Original})
	for _, file := range report.Files {
		fmt.Fprintf(c.deps.Output, "%s: %s\n", file.Path, file.Status)
	}
	return err
}
