package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"
	"unicode/utf8"
)

// AppHelpTemplate is the text template for the Default help topic.
// cli.go uses text/template to render templates. You can
// render custom help text by setting this variable.
var AppHelpTemplate = `NAME:
   {{.Name}}{{if .Usage}} - {{.Usage}}{{end}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Version}}{{if not .HideVersion}}

VERSION:
   {{.Version}}{{end}}{{end}}{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if len .Authors}}

AUTHOR{{with $length := len .Authors}}{{if ne 1 $length}}S{{end}}{{end}}:
   {{range $index, $author := .Authors}}{{if $index}}
   {{end}}{{$author}}{{end}}{{end}}{{if .VisibleCommands}}

COMMANDS:{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}{{if .VisibleFlags}}

GLOBAL OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}{{end}}{{if .Copyright}}

COPYRIGHT:
   {{.Copyright}}{{end}}
`

// CommandHelpTemplate is the text template for the command help topic.
// cli.go uses text/template to render templates. You can
// render custom help text by setting this variable.
var CommandHelpTemplate = `NAME:
   {{.HelpName}} - {{.Usage}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Category}}

CATEGORY:
   {{.Category}}{{end}}{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if .VisibleFlags}}

OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

// SubcommandHelpTemplate is the text template for the subcommand help topic.
// cli.go uses text/template to render templates. You can
// render custom help text by setting this variable.
var SubcommandHelpTemplate = `NAME:
   {{.HelpName}} - {{if .Description}}{{.Description}}{{else}}{{.Usage}}{{end}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} command{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}

COMMANDS:{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}
{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

var helpCommand = &Command{
	Name:      "help",
	Aliases:   []string{"h"},
	Usage:     "Shows a list of commands or help for one command",
	ArgsUsage: "[command]",
	Action: func(c *Context) error {
		args := c.Args()
		if args.Present() {
			return ShowCommandHelp(c, args.First())
		}

		_ = ShowAppHelp(c)
		return nil
	},
}

var helpSubcommand = &Command{
	Name:      "help",
	Aliases:   []string{"h"},
	Usage:     "Shows a list of commands or help for one command",
	ArgsUsage: "[command]",
	Action: func(c *Context) error {
		args := c.Args()
		if args.Present() {
			return ShowCommandHelp(c, args.First())
		}

		return ShowSubcommandHelp(c)
	},
}

// Prints help for the App or Command
type helpPrinter func(w io.Writer, templ string, data interface{})

// Prints help for the App or Command with custom template function.
type helpPrinterCustom func(w io.Writer, templ string, data interface{}, customFunc map[string]interface{})

// HelpPrinter is a function that writes the help output. If not set a default
// is used. The function signature is:
// func(w io.Writer, templ string, data interface{})
var HelpPrinter helpPrinter = printHelp

// HelpPrinterCustom is same as HelpPrinter but
// takes a custom function for template function map.
var HelpPrinterCustom helpPrinterCustom = printHelpCustom

// VersionPrinter prints the version for the App
var VersionPrinter = printVersion

// ShowAppHelpAndExit - Prints the list of subcommands for the app and exits with exit code.
func ShowAppHelpAndExit(c *Context, exitCode int) {
	_ = ShowAppHelp(c)
	os.Exit(exitCode)
}

// ShowAppHelp is an action that displays the help.
func ShowAppHelp(c *Context) (err error) {
	if c.App.CustomAppHelpTemplate == "" {
		HelpPrinter(c.App.Writer, AppHelpTemplate, c.App)
		return
	}
	customAppData := func() map[string]interface{} {
		if c.App.ExtraInfo == nil {
			return nil
		}
		return map[string]interface{}{
			"ExtraInfo": c.App.ExtraInfo,
		}
	}
	HelpPrinterCustom(c.App.Writer, c.App.CustomAppHelpTemplate, c.App, customAppData())
	return nil
}

// DefaultAppComplete prints the list of subcommands as the default app completion method
func DefaultAppComplete(c *Context) {
	DefaultCompleteWithFlags(nil)(c)
}

func printCommandSuggestions(commands []Command, writer io.Writer) {
	for _, command := range commands {
		if command.Hidden {
			continue
		}
		if os.Getenv("_CLI_ZSH_AUTOCOMPLETE_HACK") == "1" {
			for _, name := range command.Names() {
				_, _ = fmt.Fprintf(writer, "%s:%s\n", name, command.Usage)
			}
		} else {
			for _, name := range command.Names() {
				_, _ = fmt.Fprintf(writer, "%s\n", name)
			}
		}
	}
}

func cliArgContains(flagName string) bool {
	for _, name := range strings.Split(flagName, ",") {
		name = strings.TrimSpace(name)
		count := utf8.RuneCountInString(name)
		if count > 2 {
			count = 2
		}
		flag := fmt.Sprintf("%s%s", strings.Repeat("-", count), name)
		for _, a := range os.Args {
			if a == flag {
				return true
			}
		}
	}
	return false
}

func printFlagSuggestions(lastArg string, flags []Flag, writer io.Writer) {
	cur := strings.TrimPrefix(lastArg, "-")
	cur = strings.TrimPrefix(cur, "-")
	for _, flag := range flags {
		if bflag, ok := flag.(BoolFlag); ok && bflag.Hidden {
			continue
		}
		for _, name := range strings.Split(flag.GetName(), ",") {
			name = strings.TrimSpace(name)
			// this will get total count utf8 letters in flag name
			count := utf8.RuneCountInString(name)
			if count > 2 {
				count = 2 // resuse this count to generate single - or -- in flag completion
			}
			// if flag name has more than one utf8 letter and last argument in cli has -- prefix then
			// skip flag completion for short flags example -v or -x
			if strings.HasPrefix(lastArg, "--") && count == 1 {
				continue
			}
			// match if last argument matches this flag and it is not repeated
			if strings.HasPrefix(name, cur) && cur != name && !cliArgContains(flag.GetName()) {
				flagCompletion := fmt.Sprintf("%s%s", strings.Repeat("-", count), name)
				_, _ = fmt.Fprintln(writer, flagCompletion)
			}
		}
	}
}

func DefaultCompleteWithFlags(cmd *Command) func(c *Context) {
	return func(c *Context) {
		if len(os.Args) > 2 {
			lastArg := os.Args[len(os.Args)-2]
			if strings.HasPrefix(lastArg, "-") {
				printFlagSuggestions(lastArg, c.App.Flags, c.App.Writer)
				if cmd != nil {
					printFlagSuggestions(lastArg, cmd.Flags, c.App.Writer)
				}
				return
			}
		}
		if cmd != nil {
			printCommandSuggestions(cmd.Subcommands, c.App.Writer)
		} else {
			printCommandSuggestions(c.App.Commands, c.App.Writer)
		}
	}
}

// ShowCommandHelpAndExit - exits with code after showing help
func ShowCommandHelpAndExit(c *Context, command string, code int) {
	_ = ShowCommandHelp(c, command)
	os.Exit(code)
}

// ShowCommandHelp prints help for the given command
func ShowCommandHelp(ctx *Context, command string) error {
	// show the subcommand help for a command with subcommands
	if command == "" {
		HelpPrinter(ctx.App.Writer, SubcommandHelpTemplate, ctx.App)
		return nil
	}

	for _, c := range ctx.App.Commands {
		if c.HasName(command) {
			if c.CustomHelpTemplate != "" {
				HelpPrinterCustom(ctx.App.Writer, c.CustomHelpTemplate, c, nil)
			} else {
				HelpPrinter(ctx.App.Writer, CommandHelpTemplate, c)
			}
			return nil
		}
	}

	if ctx.App.CommandNotFound == nil {
		return Exit(fmt.Sprintf("No help topic for '%v'", command), 3)
	}

	ctx.App.CommandNotFound(ctx, command)
	return nil
}

// ShowSubcommandHelp prints help for the given subcommand
func ShowSubcommandHelp(c *Context) error {
	if c == nil {
		return nil
	}

	if c.Command != nil {
		return ShowCommandHelp(c, c.Command.Name)
	}

	return ShowCommandHelp(c, "")
}

// ShowVersion prints the version number of the App
func ShowVersion(c *Context) {
	VersionPrinter(c)
}

func printVersion(c *Context) {
	_, _ = fmt.Fprintf(c.App.Writer, "%v version %v\n", c.App.Name, c.App.Version)
}

// ShowCompletions prints the lists of commands within a given context
func ShowCompletions(c *Context) {
	a := c.App
	if a != nil && a.BashComplete != nil {
		a.BashComplete(c)
	}
}

// ShowCommandCompletions prints the custom completions for a given command
func ShowCommandCompletions(ctx *Context, command string) {
	c := ctx.App.Command(command)
	//TODO: Resolve
//<<<<<<< HEAD
	if c != nil && c.ShellComplete != nil {
		c.ShellComplete(ctx)
//=======
//	if c != nil {
//		if c.BashComplete != nil {
//			c.BashComplete(ctx)
//		} else {
//			DefaultCompleteWithFlags(c)(ctx)
//		}
//>>>>>>> master
	}

}

func printHelpCustom(out io.Writer, templ string, data interface{}, customFunc map[string]interface{}) {
	funcMap := template.FuncMap{
		"join": strings.Join,
	}
	for key, value := range customFunc {
		funcMap[key] = value
	}

	w := tabwriter.NewWriter(out, 1, 8, 2, ' ', 0)
	t := template.Must(template.New("help").Funcs(funcMap).Parse(templ))

	err := t.Execute(w, data)
	if err != nil {
		// If the writer is closed, t.Execute will fail, and there's nothing
		// we can do to recover.
		if os.Getenv("CLI_TEMPLATE_ERROR_DEBUG") != "" {
			_, _ = fmt.Fprintf(ErrWriter, "CLI TEMPLATE ERROR: %#v\n", err)
		}
		return
	}
	_ = w.Flush()
}

func printHelp(out io.Writer, templ string, data interface{}) {
	printHelpCustom(out, templ, data, nil)
}

func checkVersion(c *Context) bool {
	found := false
	for _, name := range VersionFlag.Names() {
		if c.Bool(name) {
			found = true
		}
	}
	return found
}

func checkHelp(c *Context) bool {
	found := false
	for _, name := range HelpFlag.Names() {
		if c.Bool(name) {
			found = true
		}
	}
	return found
}

func checkCommandHelp(c *Context, name string) bool {
	if c.Bool("h") || c.Bool("help") {
		_ = ShowCommandHelp(c, name)
		return true
	}

	return false
}

func checkSubcommandHelp(c *Context) bool {
	if c.Bool("h") || c.Bool("help") {
		_ = ShowSubcommandHelp(c)
		return true
	}

	return false
}

func checkShellCompleteFlag(a *App, arguments []string) (bool, []string) {
	if !a.EnableShellCompletion {
		return false, arguments
	}

	pos := len(arguments) - 1
	lastArg := arguments[pos]

	if lastArg != "--"+genCompName() {
		return false, arguments
	}

	return true, arguments[:pos]
}

func checkCompletions(c *Context) bool {
	if !c.shellComplete {
		return false
	}

	if args := c.Args(); args.Present() {
		name := args.First()
		if cmd := c.App.Command(name); cmd != nil {
			// let the command handle the completion
			return false
		}
	}

	ShowCompletions(c)
	return true
}

func checkCommandCompletions(c *Context, name string) bool {
	if !c.shellComplete {
		return false
	}

	ShowCommandCompletions(c, name)
	return true
}

func checkInitCompletion(c *Context) (bool, error) {
	if c.IsSet(InitCompletionFlag.Name) {
		shell := c.String(InitCompletionFlag.Name)
		progName := os.Args[0]
		switch shell {
		case "bash":
			fmt.Print(bashCompletionCode(progName))
			return true, nil
		case "zsh":
			fmt.Print(zshCompletionCode(progName))
			return true, nil
		default:
			return false, fmt.Errorf("--init-completion value cannot be '%s'", shell)
		}
	}
	return false, nil
}

func bashCompletionCode(progName string) string {
	var template = `_cli_bash_autocomplete() {
     local cur opts base;
     COMPREPLY=();
     cur="${COMP_WORDS[COMP_CWORD]}";
     opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --%s );
     COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) );
     return 0;
};
complete -F _cli_bash_autocomplete %s`
	return fmt.Sprintf(template, genCompName(), progName)
}

func zshCompletionCode(progName string) string {
	var template = `autoload -U compinit && compinit;
autoload -U bashcompinit && bashcompinit;`

	return template + "\n" + bashCompletionCode(progName)
}
