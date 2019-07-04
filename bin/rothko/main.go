// Copyright (C) 2018. See AUTHORS.

package main

import (
	"github.com/zeebo/rothko"
	"github.com/zeebo/rothko/external"
	"go.uber.org/zap"
)

func main() {
	// set up logging with go.uber.org/zap
	var logger, _ = zap.NewDevelopment(zap.AddCallerSkip(2))
	defer logger.Sync()
	external.Default.Logger = logger.Sugar()

	rothko.Main()
}
