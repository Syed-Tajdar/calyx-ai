package connectors

import "errors"

// ErrUnsupportedMethod is returned when the HTTP method is not allowed for a connector.
var ErrUnsupportedMethod = errors.New("connectors: method not allowed")
