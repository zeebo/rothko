// Copyright (C) 2018. See AUTHORS.

package graphite

import (
	"context"

	"github.com/zeebo/rothko/internal/typeassert"
	"github.com/zeebo/rothko/listener"
	"github.com/zeebo/rothko/registry"
)

func init() {
	registry.RegisterListener("graphite", registry.ListenerMakerFunc(
		func(ctx context.Context, config interface{}) (listener.Listener, error) {
			a := typeassert.A(config)
			lis := New(a.I("address").String())
			if err := a.Err(); err != nil {
				return nil, err
			}

			return lis, nil
		}))
}
