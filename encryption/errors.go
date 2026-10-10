package encryption

import "errors"

// Errors contain only fixed, non-secret text. Provider errors are deliberately
// not wrapped: their Error and Unwrap methods may expose credential material.
var (
	ErrAuthentication           = errors.New("encryption: authentication failed")
	ErrFormat                   = errors.New("encryption: invalid encoding")
	ErrUnsupported              = errors.New("encryption: unsupported format or suite")
	ErrUnsupportedConfiguration = errors.New("encryption: unsupported cryptographic configuration")
	ErrLimit                    = errors.New("encryption: resource limit exceeded")
	ErrKDFPolicy                = errors.New("encryption: KDF parameters outside policy")
	ErrCredential               = errors.New("encryption: invalid credential material")
	ErrKeyProvider              = errors.New("encryption: key provider failed")
	ErrRandomness               = errors.New("encryption: fresh random identity unavailable")
	ErrClosed                   = errors.New("encryption: handle closed")
	ErrWriterState              = errors.New("encryption: invalid writer lifecycle")
	ErrIdentity                 = errors.New("encryption: unexpected identity")
)
