package vmware

import (
	"context"
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/sim25"
	"github.com/vmware/govmomi/vim25"
)

// TestNewClientKeepAliveSurvivesSessionTimeout proves the Phase 0 fix: the
// SOAP session created by NewClient must NOT expire under a keepalive-idle
// window shorter than the server's session timeout. It exercises our own
// connect->wrap->login ordering (newClient), not govmomi's keepalive package
// directly (that has its own passing tests upstream) — what we're
// validating is that OUR wiring didn't get the order wrong (e.g. wrapping
// the round-tripper AFTER login would silently never start the ticker).
func TestNewClientKeepAliveSurvivesSessionTimeout(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		const (
			sessionCheckPause  = 500 * time.Millisecond
			sessionIdleTimeout = sessionCheckPause / 2
			testKeepAliveIdle  = sessionIdleTimeout / 2
		)

		if err := sim25.SetSessionTimeout(ctx, vc, sessionIdleTimeout); err != nil {
			t.Fatalf("failed to set simulator session timeout: %v", err)
		}

		c, err := newClient(ctx, Config{
			URL:      vc.URL().String(),
			Username: "user",
			Password: "pass",
			Insecure: true,
		}, testKeepAliveIdle)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		valid := func() bool {
			s, err := c.SessionManager.UserSession(ctx)
			if err != nil {
				t.Fatalf("UserSession failed: %v", err)
			}
			return s != nil
		}

		if !valid() {
			t.Fatal("expected session to be valid immediately after NewClient")
		}

		time.Sleep(sessionCheckPause) // > sessionIdleTimeout

		if !valid() {
			t.Fatal("session expired despite keepalive being idle-shorter than the server timeout — NewClient's connect/wrap/login ordering regressed")
		}
	})
}

// TestNewClientLogsOutAndStopsKeepAlive proves Close() actually logs out —
// the keepalive handler stops its own ticker on observing the logout
// round-trip (govmomi behavior), so this also indirectly confirms Close
// doesn't leak a running ticker goroutine.
func TestNewClientLogsOutAndStopsKeepAlive(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		c, err := newClient(ctx, Config{
			URL:      vc.URL().String(),
			Username: "user",
			Password: "pass",
			Insecure: true,
		}, keepAliveIdle)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		if err := c.Close(ctx); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		s, err := c.SessionManager.UserSession(ctx)
		if err == nil && s != nil {
			t.Fatal("expected session to be invalid after Close/Logout")
		}
	})
}
