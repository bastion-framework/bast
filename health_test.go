package bast_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bastion-framework/bast"
)

func TestHealthCheck_Liveness_AlwaysAlive(t *testing.T) {
	app := bast.New(bast.Config{
		Health: &bast.HealthConfig{
			LivePath:  "/health",
			ReadyPath: "/ready",
		},
	})

	resp := app.LivenessHandler()(bast.NewTestCtxWithPath("/health"))
	if resp.Status() != 200 {
		t.Errorf("liveness status = %d, want 200", resp.Status())
	}
	if !containsBytes(resp.Body(), "alive") {
		t.Errorf("liveness body should contain 'alive', got %s", resp.Body())
	}
}

func TestHealthCheck_Readiness_AllHealthy(t *testing.T) {
	app := bast.New(bast.Config{
		Health: &bast.HealthConfig{
			LivePath:  "/health",
			ReadyPath: "/ready",
			Checks: []bast.HealthCheck{
				bast.CustomCheck("db", func(ctx context.Context) error { return nil }),
			},
		},
	})

	resp := app.ReadinessHandler()(bast.NewTestCtxWithPath("/ready"))
	if resp.Status() != 200 {
		t.Errorf("readiness status = %d, want 200 when all healthy", resp.Status())
	}
	if !containsBytes(resp.Body(), "healthy") {
		t.Errorf("body should contain 'healthy', got %s", resp.Body())
	}
}

func TestHealthCheck_Readiness_Degraded(t *testing.T) {
	app := bast.New(bast.Config{
		Health: &bast.HealthConfig{
			LivePath:  "/health",
			ReadyPath: "/ready",
			Checks: []bast.HealthCheck{
				bast.CustomCheck("db", func(ctx context.Context) error {
					return errors.New("connection refused")
				}),
			},
		},
	})

	resp := app.ReadinessHandler()(bast.NewTestCtxWithPath("/ready"))
	if resp.Status() != 503 {
		t.Errorf("readiness status = %d, want 503 when degraded", resp.Status())
	}
	if !containsBytes(resp.Body(), "degraded") {
		t.Errorf("body should contain 'degraded', got %s", resp.Body())
	}
}

func TestHealthCheck_Readiness_CheckDetail(t *testing.T) {
	app := bast.New(bast.Config{
		Health: &bast.HealthConfig{
			LivePath:  "/health",
			ReadyPath: "/ready",
			Checks: []bast.HealthCheck{
				bast.CustomCheck("redis", func(ctx context.Context) error { return nil }),
			},
		},
	})

	resp := app.ReadinessHandler()(bast.NewTestCtxWithPath("/ready"))
	if !containsBytes(resp.Body(), "redis") {
		t.Errorf("body should contain check name 'redis', got %s", resp.Body())
	}
}

func containsBytes(b []byte, s string) bool {
	return len(b) > 0 && string(b) != "" &&
		len(s) > 0 && indexOf(b, []byte(s)) >= 0
}

func indexOf(haystack, needle []byte) int {
	for i := range haystack {
		if i+len(needle) <= len(haystack) {
			match := true
			for j := range needle {
				if haystack[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}