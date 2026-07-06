package bast

import (
	"context"
	"time"

	"github.com/bastion-framework/bast/internal/jsonx"
)

// HealthConfig configures the built-in health check endpoints.
type HealthConfig struct {
	LivePath  string        // liveness probe path, e.g. "/health"
	ReadyPath string        // readiness probe path, e.g. "/ready"
	Checks    []HealthCheck // dependency checks run on /ready
}

// HealthCheck is a named dependency check run on the readiness endpoint.
type HealthCheck struct {
	Name string
	fn   func(ctx context.Context) error
}

// CustomCheck builds a HealthCheck from a name and a check function.
func CustomCheck(name string, fn func(ctx context.Context) error) HealthCheck {
	return HealthCheck{Name: name, fn: fn}
}

// checkResult holds the outcome of a single health check.
// Deliberately no error detail: /ready is typically unauthenticated and a
// failing dependency error can carry DSNs, hostnames, or internal IPs.
// The detail is logged server-side instead.
type checkResult struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
}

// LivenessHandler returns a HandlerFunc for the liveness probe.
// Always returns 200 as long as the process is alive — never checks dependencies.
func (a *App) LivenessHandler() HandlerFunc {
	return func(ctx *Ctx) Response {
		body, _ := jsonx.Marshal(map[string]string{"status": "alive"})
		return newRawResponse(200, "application/json", body)
	}
}

// ReadinessHandler returns a HandlerFunc for the readiness probe.
// Returns 200 if all checks pass, 503 if any check fails.
func (a *App) ReadinessHandler() HandlerFunc {
	checks := a.cfg.Health.Checks
	return func(ctx *Ctx) Response {
		results := make(map[string]checkResult, len(checks))
		allHealthy := true

		for _, c := range checks {
			start := time.Now()
			err := c.fn(ctx.Context())
			lat := time.Since(start)

			if err != nil {
				allHealthy = false
				a.logger.Error("readiness check %q failed: %v", c.Name, err)
				results[c.Name] = checkResult{
					Status:  "degraded",
					Latency: lat.String(),
				}
			} else {
				results[c.Name] = checkResult{
					Status:  "healthy",
					Latency: lat.String(),
				}
			}
		}

		overall := "healthy"
		status := 200
		if !allHealthy {
			overall = "degraded"
			status = 503
		}

		body, _ := jsonx.Marshal(map[string]any{
			"status": overall,
			"checks": results,
		})
		return newRawResponse(status, "application/json", body)
	}
}

// registerHealthRoutes mounts the health endpoints if configured.
func (a *App) registerHealthRoutes() {
	if a.cfg.Health == nil {
		return
	}
	h := a.cfg.Health
	if h.LivePath != "" {
		a.router.Add("GET", h.LivePath, a.LivenessHandler())
	}
	if h.ReadyPath != "" {
		a.router.Add("GET", h.ReadyPath, a.ReadinessHandler())
	}
}