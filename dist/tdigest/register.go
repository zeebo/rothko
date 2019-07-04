// Copyright (C) 2018. See AUTHORS.

package tdigest

import (
	"context"

	"github.com/zeebo/rothko/dist"
	"github.com/zeebo/rothko/internal/typeassert"
	"github.com/zeebo/rothko/registry"
)

func init() {
	registry.RegisterDistribution("tdigest", registry.DistributionMakerFunc(
		func(ctx context.Context, config interface{}) (dist.Params, error) {
			if config == nil {
				return Params{}, nil
			}

			a := typeassert.A(config)
			params := Params{
				Compression: a.I("compression").Float64(),
			}
			if err := a.Err(); err != nil {
				return nil, err
			}

			return params, nil
		}))
}
