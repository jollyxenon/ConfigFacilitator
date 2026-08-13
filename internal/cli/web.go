package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/web"
)

// newWebCommand constructs the local Web UI server command.
func newWebCommand(context *commandContext) *cobra.Command {
	var port int
	command := &cobra.Command{
		Use:     "web",
		Short:   "Serve the local Web UI",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc web\n  cfgfc web --port 49632",
		RunE: func(command *cobra.Command, args []string) error {
			return runWeb(context, port)
		},
	}
	command.Flags().IntVar(&port, "port", 49631, "Listen on this localhost port")
	return command
}

// runWeb serves the embedded UI until the process is interrupted.
func runWeb(commandContext *commandContext, port int) error {
	handler, err := web.NewHandler(web.WebDependencies{
		HomeDir:         commandContext.dependencies.HomeDir,
		Environment:     commandContext.dependencies.Environment,
		OperatingSystem: commandContext.dependencies.OperatingSystem,
		Stdout:          commandContext.dependencies.Stdout,
	})
	if err != nil {
		return NewPersistenceError("web_init", err.Error(), err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return NewPersistenceError("web_listen", fmt.Sprintf("listen on %s: %v", address, err), err)
	}
	server := &http.Server{Handler: handler}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupts)

	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
			return
		}
		serveErrors <- nil
	}()

	_, _ = fmt.Fprintf(commandContext.dependencies.Stdout, "Web UI listening at http://%s\n", address)
	_, _ = fmt.Fprintf(commandContext.dependencies.Stdout, "Press Ctrl-C to stop.\n")

	select {
	case serveErr := <-serveErrors:
		if serveErr != nil {
			return NewPersistenceError("web_serve", serveErr.Error(), serveErr)
		}
		return nil
	case <-interrupts:
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		return nil
	}
}
