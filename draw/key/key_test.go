// Copyright (C) 2018. See AUTHORS.

package key

import (
	"image/png"
	"os"
	"testing"

	"github.com/zeebo/rothko/draw"
	"github.com/zeebo/rothko/draw/colors"
	"github.com/zeebo/rothko/internal/assert"
)

func saveImage(t *testing.T, name string, out *draw.RGB) {
	if false { // set to false to save images
		return
	}

	fh, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	assert.NoError(t, err)
	defer fh.Close()

	assert.NoError(t, png.Encode(fh, out.AsImage()))
}

func TestDraw(t *testing.T) {
	saveImage(t, "key.png", Draw(nil, Options{
		Colors: colors.Viridis,
		Width:  10,
		Height: 300,
	}))
}
