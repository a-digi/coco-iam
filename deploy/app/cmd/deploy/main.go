// deploy is the CLI entry point for the coco-iam-deploy tool.
//
// Subcommands:
//
//	deploy     run the full Preflight → Upload → RenderEnv →
//	           StartProcesses → HealthCheck lifecycle (default
//	           if no subcommand is given)
//	status     show what's currently deployed on the target
//	rollback   swap `current` back to the previous release
//	validate   parse + validate the YAML without connecting
//
// Every subcommand reads the same YAML file (--config, default
// deploy.yaml). Keeping one YAML across all subcommands lets
// CI pipelines share configuration between "deploy" and
// "rollback" steps without re-templating.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/a-digi/coco-deploy/config"
	"github.com/a-digi/coco-deploy/engine"
	"github.com/a-digi/coco-deploy/runner"

	// Import engines for their self-registration side effects.
	_ "github.com/a-digi/coco-deploy/engine/ssh"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "deploy:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	sub, rest := parseSubcommand(args)
	switch sub {
	case "", "deploy":
		return cmdDeploy(rest)
	case "status":
		return cmdStatus(rest)
	case "rollback":
		return cmdRollback(rest)
	case "validate":
		return cmdValidate(rest)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand %q (engines registered: %v)", sub, engine.KnownEngines())
	}
}

func parseSubcommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return args[0], args[1:]
}

func cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	cfgPath := fs.String("config", "deploy.yaml", "path to the deploy YAML")
	noRollback := fs.Bool("no-rollback", false, "skip automatic Engine.Rollback on failure")
	artifactName := fs.String("artifact", "", "deploy only this named artifact (skips all others)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	f, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	stamp := newVersionStamp()
	eng, rel, err := f.Build(stamp)
	if err != nil {
		return err
	}
	defer eng.Close()

	if *artifactName != "" {
		filtered := rel.Artifacts[:0]
		for _, a := range rel.Artifacts {
			if a.Name == *artifactName {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) == 0 {
			names := make([]string, 0, len(rel.Artifacts))
			for _, a := range rel.Artifacts {
				names = append(names, a.Name)
			}
			return fmt.Errorf("--artifact %q not found; available: %s", *artifactName, strings.Join(names, ", "))
		}
		rel.Artifacts = filtered
	}

	ctx, cancel := signalAwareContext()
	defer cancel()

	r := &runner.Runner{
		Engine:  eng,
		Log:     stdLogger(),
		Options: runner.Options{NoRollback: *noRollback},
	}
	if err := r.Deploy(ctx, rel); err != nil {
		return err
	}
	if pruner, ok := eng.(interface {
		PruneOldReleases(ctx context.Context) error
	}); ok {
		if err := pruner.PruneOldReleases(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: prune old releases: %v\n", err)
		}
	}
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	cfgPath := fs.String("config", "deploy.yaml", "path to the deploy YAML")
	if err := fs.Parse(args); err != nil {
		return err
	}
	f, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	eng, _, err := f.Build("status-probe")
	if err != nil {
		return err
	}
	defer eng.Close()

	ctx, cancel := signalAwareContext()
	defer cancel()

	report, err := eng.Status(ctx)
	if err != nil {
		return err
	}
	printStatus(report)
	return nil
}

func cmdRollback(args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	cfgPath := fs.String("config", "deploy.yaml", "path to the deploy YAML")
	if err := fs.Parse(args); err != nil {
		return err
	}
	f, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	eng, rel, err := f.Build("rollback-" + newVersionStamp())
	if err != nil {
		return err
	}
	defer eng.Close()

	ctx, cancel := signalAwareContext()
	defer cancel()

	if err := eng.Preflight(ctx, rel); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := eng.Rollback(ctx, rel); err != nil {
		if errors.Is(err, engine.ErrRollbackUnsupported) {
			return fmt.Errorf("rollback: %w (no previous release captured — is this the first deploy?)", err)
		}
		return err
	}
	fmt.Println("rollback: complete")
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	cfgPath := fs.String("config", "deploy.yaml", "path to the deploy YAML")
	if err := fs.Parse(args); err != nil {
		return err
	}
	f, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	eng, rel, err := f.Build(newVersionStamp())
	if err != nil {
		return err
	}
	defer eng.Close()

	fmt.Printf("config: ok\n")
	fmt.Printf("  engine:    %s\n", eng.Name())
	fmt.Printf("  release:   %s %s\n", rel.Name, rel.Version)
	fmt.Printf("  artifacts: %d\n", len(rel.Artifacts))
	fmt.Printf("  processes: %d\n", len(rel.Processes))
	fmt.Printf("  env vars:  %d\n", len(rel.Env))
	if rel.HealthCheck.URL != "" {
		fmt.Printf("  health:    %s\n", rel.HealthCheck.URL)
	}
	return nil
}

func printUsage() {
	fmt.Println(`coco-deploy — YAML-driven engine-pluggable deployment tool

Usage:
  deploy [subcommand] [--config deploy.yaml] [flags]

Subcommands:
  deploy      run the full deploy lifecycle (default)
  status      show what's currently running on the target
  rollback    swap current → previous release and restart processes
  validate    parse + validate the YAML without connecting

Flags:
  --config       path to the deploy YAML (default: deploy.yaml)
  --no-rollback  skip Engine.Rollback on failure (deploy only)
  --artifact     deploy only the named artifact, skip all others (deploy only)`)
}

func printStatus(r engine.StatusReport) {
	fmt.Printf("engine:   %s\n", r.Engine)
	fmt.Printf("target:   %s\n", r.Target)
	fmt.Printf("current:  %s\n", r.CurrentVersion)
	fmt.Printf("previous: %s\n", r.PreviousVersion)
	if r.DeployedAt != "" {
		fmt.Printf("deployed: %s\n", r.DeployedAt)
	}
	for k, v := range r.Notes {
		fmt.Printf("  %s: %s\n", k, v)
	}
}

func newVersionStamp() string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(buf)
}

func signalAwareContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func stdLogger() runner.Logger {
	return &stdLoggerAdapter{l: log.New(os.Stderr, "", log.LstdFlags)}
}

type stdLoggerAdapter struct{ l *log.Logger }

func (s *stdLoggerAdapter) Infof(f string, a ...any)  { s.l.Printf(f, a...) }
func (s *stdLoggerAdapter) Errorf(f string, a ...any) { s.l.Printf(f, a...) }
