//go:build !linux || !manual_rekey_interruption

package encstore

import "context"

func rekeyPublicationObserve(context.Context, rekeyPublicationPoint, string, []byte, []byte) {}
