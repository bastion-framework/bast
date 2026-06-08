package bast

// Guard is a pre-handler check. Return nil to allow, return an error to block.
// Guards run before the handler and short-circuit on the first non-nil error.
type Guard interface {
	Check(ctx *Ctx) error
}

// GuardFunc is a function adapter for Guard.
type GuardFunc func(ctx *Ctx) error

func (f GuardFunc) Check(ctx *Ctx) error { return f(ctx) }

// SecurityScheme describes an OpenAPI security scheme declared by a guard.
type SecurityScheme struct {
	Type         string // "http", "apiKey", "oauth2"
	Scheme       string // "bearer", "basic"
	BearerFormat string // "JWT"
	Description  string
}

// SecuredGuard is a Guard that also declares its OpenAPI security scheme.
// Bast uses this to populate securitySchemes in the OpenAPI spec automatically.
type SecuredGuard interface {
	Guard
	SecurityScheme() SecurityScheme
}