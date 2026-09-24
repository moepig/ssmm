package cli

import (
	"strings"
	"testing"
)

// 接続コマンドの短縮フラグが AWS プロファイル、リージョン、タグ、ssmm プロファイルに一貫して対応することを検証する。
func TestConnectionShorthands(t *testing.T) {
	for _, name := range []string{"", "list", "connect", "ssh", "scp", "proxy"} {
		root := New(Dependencies{})
		command := root
		if name != "" {
			var err error
			command, _, err = root.Find([]string{name})
			if err != nil {
				t.Fatal(err)
			}
		}
		for short, long := range map[string]string{"p": "profile", "r": "region", "t": "tag", "s": "ssmm-profile"} {
			flag := command.Flags().ShorthandLookup(short)
			if flag == nil || flag.Name != long {
				t.Fatalf("%s does not expose -%s for --%s", name, short, long)
			}
		}
	}
}

// ターゲット選択方式を切り替える廃止済みフラグを各検索コマンドが公開しない。
func TestNonInteractiveFlagIsRemoved(t *testing.T) {
	for _, name := range []string{"", "list", "connect", "ssh", "scp"} {
		root := New(Dependencies{})
		command := root
		if name != "" {
			var err error
			command, _, err = root.Find([]string{name})
			if err != nil {
				t.Fatal(err)
			}
		}
		if command.Flags().Lookup("non-interactive") != nil {
			t.Errorf("%s still exposes --non-interactive", name)
		}
	}
}

// 設定の保存・SSH 設定の生成でも -s が ssmm プロファイルの指定として解釈されることを検証する。
func TestSettingsProfileShorthandPassesThroughCobra(t *testing.T) {
	for _, args := range [][]string{
		{"init", "-s", "prod", "-p", "company", "-r", "us-east-1"},
		{"ssh-config", "create", "-s", "prod"},
		{"ssh-config", "delete", "-s", "prod"},
	} {
		command := New(Dependencies{})
		command.SetArgs(args)
		err := command.Execute()
		if err != nil && strings.Contains(err.Error(), "unknown shorthand flag") {
			t.Fatalf("Cobra rejected a documented shorthand for %v: %v", args, err)
		}
	}
}
