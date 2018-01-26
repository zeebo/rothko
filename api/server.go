// Copyright (C) 2018. See AUTHORS.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/spacemonkeygo/rothko/data"
	"github.com/spacemonkeygo/rothko/data/dists"
	"github.com/spacemonkeygo/rothko/data/dists/tdigest"
	"github.com/spacemonkeygo/rothko/disk"
	"github.com/spacemonkeygo/rothko/draw/colors"
	"github.com/spacemonkeygo/rothko/draw/graph"
	"github.com/spacemonkeygo/rothko/draw/merge"
	"github.com/zeebo/errs"
)

// Server is an http.Handler that can serve responses for a frontend.
type Server struct {
	di disk.Disk
}

// New returns a new Server.
func New(di disk.Disk) *Server {
	return &Server{
		di: di,
	}
}

// ServeHTTP implements the http.Handler interface for the server. It just
// looks at the method and last path component to route.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	tracker := &respTracker{ResponseWriter: w, wrote: false}
	if err := s.serveHTTP(req.Context(), w, req); err != nil {
		if !tracker.wrote {
			w.WriteHeader(getStatusCode(err))
			fmt.Fprintf(w, "%+v\n", err)
		}
	}
}

// serveHTTP is a little wrapper for making error handling easier.
func (s *Server) serveHTTP(ctx context.Context, w http.ResponseWriter,
	req *http.Request) (err error) {

	if req.Method != "GET" {
		return errMethodNotAllowed.New("%s", req.Method)
	}

	switch _, last := path.Split(req.URL.Path); last {
	case "metrics":
		return s.serveMetrics(ctx, w, req)

	case "render":
		return s.serveRender(ctx, w, req)

	default:
		return errNotFound.New("path: %q", last)
	}
}

// TODO(jeff): maybe we need a query metrics call? maybe this server also
// handles auto complete results? if so we can just perioidcally cache all
// the metrics here.

// serveMetrics serves up the set of metrics that are available. It streams
// the response.
func (s *Server) serveMetrics(ctx context.Context, w http.ResponseWriter,
	req *http.Request) (err error) {

	w.Header().Set("Content-Type", "application/json")

	if _, err := io.WriteString(w, "["); err != nil {
		return errs.Wrap(err)
	}

	enc := json.NewEncoder(w)
	first := true

	err = s.di.Metrics(ctx, func(name string) error {
		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return errs.Wrap(err)
			}
		}
		return errs.Wrap(enc.Encode(name))
	})
	if err != nil {
		return errs.Wrap(err)
	}

	if _, err := io.WriteString(w, "]"); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// serveRender serves either a png of the graph, or a json encoded set of
// columns, and the earliest data so that the frontend can draw the graph.
func (s *Server) serveRender(ctx context.Context, w http.ResponseWriter,
	req *http.Request) (err error) {

	// get the render parameters
	metric := req.FormValue("metric")
	if metric == "" {
		return errBadRequest.New("metric required")
	}

	width := getInt(req.FormValue("width"), 1000)
	height := getInt(req.FormValue("height"), 360)
	now := getInt64(req.FormValue("now"), time.Now().UnixNano())
	dur := getDuration(req.FormValue("duration"), 24*time.Hour)
	samples := getInt(req.FormValue("samples"), 30)
	compression := getFloat64(req.FormValue("compression"), 5)
	stop_before := now - dur.Nanoseconds()

	// set up some state for the query
	var measured graph.Measured
	var earliest []byte
	measure_opts := graph.MeasureOptions{
		Now:      now,
		Duration: dur,
		Width:    width,
		Height:   height,
	}

	merger := merge.New(merge.Options{
		Samples:  samples,
		Now:      now,
		Duration: dur,
		Params:   tdigest.Params{Compression: compression},
	})

	// run the query
	err = s.di.Query(ctx, metric, now, nil,
		func(ctx context.Context, start, end int64, buf []byte) (
			bool, error) {

			// get the record ready
			var rec data.Record
			if err := rec.Unmarshal(buf); err != nil {
				return false, errs.Wrap(err)
			}

			// if we don't have an earliest yet, keep it around and set it up
			if measure_opts.Earliest == nil {
				dist, err := dists.Load(rec)
				if err != nil {
					return false, errs.Wrap(err)
				}

				earliest = append(earliest[:0], buf...)
				measure_opts.Earliest = dist
				measured = graph.Measure(ctx, measure_opts)
				merger.SetWidth(measured.Width)
			}

			// push in the record
			if err := merger.Push(ctx, rec); err != nil {
				return false, errs.Wrap(err)
			}

			// keep going until we need to stop based on the duration
			return end < stop_before, nil
		})
	if err != nil {
		return errs.Wrap(err)
	}

	// grab the columns
	cols, err := merger.Finish(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	// if it's json, encode it out
	if req.Header.Get("Accept") == "application/json" {
		type D = map[string]interface{}
		return errs.Wrap(json.NewEncoder(w).Encode(D{
			"Columns":  cols,
			"Earliest": earliest,
		}))
	}

	// if we never got an earliest, we need to measure without it to get the
	// axes ready.
	if measure_opts.Earliest == nil {
		measured = graph.Measure(ctx, measure_opts)
	}

	// draw the graph
	out := measured.Draw(ctx, graph.DrawOptions{
		Canvas:  nil,
		Columns: cols,
		Colors:  colors.Viridis,
	})

	// encode it out as a png
	return errs.Wrap(png.Encode(w, &image.RGBA{
		Pix:    out.Pix,
		Stride: out.Stride,
		Rect:   image.Rect(0, 0, out.Width, out.Height),
	}))
}
