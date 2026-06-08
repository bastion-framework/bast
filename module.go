package bast

import "context"

// Module is a portable, self-contained unit of functionality.
// It owns its routes, guards, middleware, and optionally sub-modules.
type Module struct {
	// Prefix is the URL prefix for all routes in this module.
	Prefix string

	// Controller declares this module's routes.
	Controller Controller

	// Middleware scoped to every route in this module.
	// Runs after global middleware, before route-level middleware.
	Middleware []MiddlewareFunc

	// Guards scoped to every route in this module.
	// Runs after global guards, before route-level guards.
	Guards []Guard

	// Nested sub-modules. Inherits parent prefix.
	Modules []Module

	// Doc metadata for Swagger tag group.
	Doc ModuleDoc
}

// ModuleDoc holds human-readable metadata for OpenAPI tag groups.
type ModuleDoc struct {
	Name        string
	Description string
}

// Hooks is an optional interface modules may implement to hook into the app lifecycle.
type Hooks interface {
	OnInit(ctx context.Context) error
	OnReady()
	OnShutdown(ctx context.Context) error
}