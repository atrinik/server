// Command atrinik-server is the clean-room authoritative Atrinik server shell.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/atrinik/server/internal/app"
	"github.com/atrinik/server/internal/buildinfo"
	"github.com/atrinik/server/internal/config"
	"github.com/atrinik/server/internal/observability"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		return errors.New("usage: atrinik-server version|config|serve")
	}
	switch arguments[0] {
	case "version":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(buildinfo.Current())
	case "config":
		settings, err := parseConfig(arguments[1:], stderr)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(settings.Redacted())
	case "serve":
		return serve(arguments[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func parseConfig(arguments []string, stderr io.Writer) (config.Config, error) {
	settings := config.Default()
	flags := flag.NewFlagSet("atrinik-server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&settings.ListenAddress, "listen", settings.ListenAddress, "gameplay listener address")
	flags.StringVar(&settings.StateDirectory, "state", settings.StateDirectory, "wrapper-owned mutable state directory")
	flags.StringVar(&settings.LogFormat, "log-format", settings.LogFormat, "json or human")
	flags.IntVar(&settings.QueueCapacity, "queue-capacity", settings.QueueCapacity, "bounded simulation queue capacity")
	flags.IntVar(&settings.CommandsPerTick, "commands-per-tick", settings.CommandsPerTick, "bounded simulation work per tick")
	flags.DurationVar(&settings.ShutdownTimeout, "shutdown-timeout", settings.ShutdownTimeout, "bounded graceful shutdown deadline")
	settings.AdminToken = os.Getenv("ATRINIK_ADMIN_TOKEN")
	if err := flags.Parse(arguments); err != nil {
		return config.Config{}, err
	}
	if flags.NArg() != 0 {
		return config.Config{}, errors.New("unexpected positional arguments")
	}
	return settings, settings.Validate()
}

func serve(arguments []string, stdout, stderr io.Writer) error {
	settings, err := parseConfig(arguments, stderr)
	if err != nil {
		return err
	}
	logger, err := observability.NewLogger(stdout, settings.LogFormat)
	if err != nil {
		return err
	}
	runtime := app.New(settings, logger)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	logger.Event(ctx, slog.LevelInfo, "lifecycle", "server.foundation-started", "server shell started; gameplay services are not yet ready")
	<-ctx.Done()
	shutdownContext, stop := context.WithTimeout(context.Background(), settings.ShutdownTimeout)
	defer stop()
	return runtime.Shutdown(shutdownContext)
}
