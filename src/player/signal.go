/* signal.go
 *
 * Signal Handlers
 */

package main

import (
	o "orchestra"
	"os"
	"os/signal"
	"syscall"
)

// handle the signals.  By default, we ignore everything, but the
// three terminal signals, HUP, INT, TERM, we want to explicitly
// handle.
func signalHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	for {
		sig := <-c

		ux, ok := sig.(syscall.Signal)
		if !ok {
			o.Warn("Couldn't handle signal %s, coercion failed", sig)
			continue
		}

		switch ux {
		case syscall.SIGHUP:
			o.Info("Reloading configuration...")
			reloadScores <- 1
		case syscall.SIGINT, syscall.SIGTERM:
			os.Exit(0)
		}
	}

}

func init() {
	go signalHandler()
}
