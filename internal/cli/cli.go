package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"frp-helper/internal/app"
	"frp-helper/internal/model"
)

type Runner struct {
	app *app.App
	out io.Writer
	err io.Writer
}

func New(application *app.App, out, err io.Writer) *Runner {
	return &Runner{app: application, out: out, err: err}
}

func (r *Runner) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		r.printUsage()
		return nil
	}

	switch args[0] {
	case "apply":
		return r.runApply(ctx, args[1:])
	case "install":
		return r.runInstall(ctx, args[1:])
	case "run":
		return r.runRun(ctx, args[1:])
	case "serve":
		return r.app.Serve(ctx)
	case "start":
		return r.app.Start(ctx)
	case "stop":
		return r.app.Stop(ctx)
	case "restart":
		return r.app.Restart(ctx)
	case "status":
		return r.app.Status(ctx)
	case "endpoints":
		return r.app.Endpoints(ctx)
	case "startup":
		return r.runStartup(ctx, args[1:])
	case "package":
		return r.runPackage(ctx, args[1:])
	case "purge":
		return r.runPurge(ctx, args[1:])
	case "service":
		return r.runService(ctx, args[1:])
	case "help", "--help", "-h":
		r.printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (r *Runner) runApply(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(r.err)
	filePath := fs.String("f", "", "path to manifest json")
	merge := fs.Bool("merge", false, "merge with existing manifest")
	replace := fs.Bool("replace", false, "replace existing manifest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *filePath == "" {
		return fmt.Errorf("apply requires -f access.json")
	}
	if *merge && *replace {
		return fmt.Errorf("apply accepts only one of --merge or --replace")
	}

	mode := app.ApplyReplace
	if *merge {
		mode = app.ApplyMerge
	}
	if *replace {
		mode = app.ApplyReplace
	}

	return r.app.Apply(ctx, *filePath, mode)
}

func (r *Runner) runInstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(r.err)
	version := fs.String("version", model.DefaultFRPCVersion, "frpc version")
	archive := fs.String("archive", "", "path to local frp archive")
	baseURL := fs.String("base-url", "", "base URL or template for downloads")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return r.app.Install(ctx, app.InstallOptions{
		Version:     *version,
		ArchivePath: *archive,
		BaseURL:     *baseURL,
	})
}

func (r *Runner) runRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(r.err)
	filePath := fs.String("f", "", "path to manifest json")
	merge := fs.Bool("merge", false, "merge with existing manifest")
	replace := fs.Bool("replace", false, "replace existing manifest")
	enableStartup := fs.Bool("enable-startup", false, "enable login startup")
	disableStartup := fs.Bool("disable-startup", false, "disable login startup")
	foreground := fs.Bool("foreground", false, "run in foreground and stream frpc logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *merge && *replace {
		return fmt.Errorf("run accepts only one of --merge or --replace")
	}

	mode := app.ApplyReplace
	if *merge {
		mode = app.ApplyMerge
	}
	return r.app.Run(ctx, app.RunOptions{
		ManifestPath:   *filePath,
		ApplyMode:      mode,
		EnableStartup:  *enableStartup,
		DisableStartup: *disableStartup,
		Foreground:     *foreground,
	})
}

func (r *Runner) runPackage(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	fs.SetOutput(r.err)
	filePath := fs.String("f", "", "path to manifest json")
	outputDir := fs.String("o", "", "output bundle directory")
	enableStartup := fs.Bool("startup", true, "enable login startup on first run in the bundle")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return r.app.Package(ctx, app.PackageOptions{
		ManifestPath:  *filePath,
		OutputDir:     *outputDir,
		EnableStartup: *enableStartup,
	})
}

func (r *Runner) runPurge(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("purge", flag.ContinueOnError)
	fs.SetOutput(r.err)
	withBin := fs.Bool("with-bin", false, "delete installed frpc binaries")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return r.app.Purge(ctx, *withBin)
}

func (r *Runner) runService(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("service requires a subcommand: list|enable|disable|remove")
	}

	switch args[0] {
	case "list":
		return r.app.ServiceList(ctx)
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("service enable requires <service-key>")
		}
		return r.app.SetServiceDisabled(ctx, strings.TrimSpace(args[1]), false)
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("service disable requires <service-key>")
		}
		return r.app.SetServiceDisabled(ctx, strings.TrimSpace(args[1]), true)
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("service remove requires <service-key>")
		}
		return r.app.RemoveService(ctx, strings.TrimSpace(args[1]))
	default:
		return fmt.Errorf("unknown service subcommand %q", args[0])
	}
}

func (r *Runner) runStartup(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("startup requires a subcommand: status|enable|disable")
	}

	switch args[0] {
	case "status":
		return r.app.StartupStatus(ctx)
	case "enable":
		return r.app.StartupEnable(ctx)
	case "disable":
		return r.app.StartupDisable(ctx)
	default:
		return fmt.Errorf("unknown startup subcommand %q", args[0])
	}
}

func (r *Runner) printUsage() {
	fmt.Fprintln(r.out, "frp-helper commands:")
	fmt.Fprintln(r.out, "  apply -f access.json [--merge|--replace]")
	fmt.Fprintln(r.out, "  install [--version v0.68.0] [--archive path|--base-url url]")
	fmt.Fprintln(r.out, "  run [-f access.json] [--merge|--replace] [--enable-startup|--disable-startup] [--foreground]")
	fmt.Fprintln(r.out, "  start | stop | restart | status | endpoints")
	fmt.Fprintln(r.out, "  startup status|enable|disable")
	fmt.Fprintln(r.out, "  package [-f access.json] [-o dist/bundle] [--startup=true]")
	fmt.Fprintln(r.out, "  purge [--with-bin]")
	fmt.Fprintln(r.out, "  service list|enable <service-key>|disable <service-key>|remove <service-key>")
}
