package bast

// Controller is the interface every controller must satisfy.
// Routes() declares the handler mappings. That is the only contract.
type Controller interface {
	Routes() []Route
}