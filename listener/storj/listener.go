// Copyright (C) 2018. See AUTHORS.

package storj

import (
	"context"

	"github.com/zeebo/rothko/data"
	"storj.io/storj/pkg/telemetry"
)

// Listener implements the listener.Listener for the graphite wire protocol.
type Listener struct {
	address string
}

// New returns a Listener that when Run will listen on the provided address.
func New(address string) *Listener {
	return &Listener{
		address: address,
	}
}

// Run listens on the address and writes all of the metrics to the writer.
func (l *Listener) Run(ctx context.Context, w *data.Writer) (err error) {
	s, err := telemetry.Listen(l.address)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Serve(ctx, telemetry.HandlerFunc(func(
		application, instance string, key []byte, val float64) {
		w.Add(ctx, application+"."+string(key), val, []byte(instance))
	}))
}
