package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Shopify/go-lua"
	"github.com/spf13/cobra"
	"github.com/treeverse/lakefs/pkg/actions"
	lualibs "github.com/treeverse/lakefs/pkg/actions/lua"
	luautil "github.com/treeverse/lakefs/pkg/actions/lua/util"
)

var luaCmd = &cobra.Command{
	Use:    "lua",
	Short:  "Lua related commands for dev/test scripting",
	Hidden: true,
}

var luaRunCmd = &cobra.Command{
	Use:   "run [file.lua] [args|@json filename]",
	Short: "Run lua code locally for testing. Use stdin when no file is given",
	Run: func(cmd *cobra.Command, args []string) {
		var filename string
		if len(args) > 0 {
			filename = args[0]
		}

		ctx := cmd.Context()
		l := lua.NewStateEx()
		lualibs.OpenSafe(l, ctx, lualibs.OpenSafeConfig{NetHTTPEnabled: true}, os.Stdout)
		loadArgs(l, args)
		if err := lua.DoFile(l, filename); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
	},
}

// loadArgs load args to our lua state.
// The first argument is optional filename, the second is either args or @filename
func loadArgs(l *lua.State, args []string) {
	if len(args) > 1 && strings.HasPrefix(args[1], "@") {
		// load args from json file - args[1] is @filename
		argsFilename := args[1][1:]
		argsData, err := os.ReadFile(argsFilename)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error reading args file (%s): %s\n", argsFilename, err)
			os.Exit(1)
		}
		var argsMap map[string]interface{}
		if err := json.Unmarshal(argsData, &argsMap); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error parsing args file (%s): %s\n", argsFilename, err)
			os.Exit(1)
		}
		parsedArgs, err := actions.DescendArgs(argsMap)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error parsing args: %s\n", err)
			os.Exit(1)
		}
		m, ok := parsedArgs.(map[string]interface{})
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "error parsing args, got wrong type: %T", parsedArgs)
			os.Exit(1)
		}

		luautil.DeepPush(l, m)
	} else {
		// add cmd line args as lua args
		luautil.DeepPush(l, args[1:])
	}
	l.SetGlobal("args")
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(luaCmd)
	luaCmd.AddCommand(luaRunCmd)
}
