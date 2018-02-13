// Copyright (C) 2018. See AUTHORS.

// query is a command line interface to querying metrics.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spacemonkeygo/rothko/api"
	"github.com/spacemonkeygo/rothko/database/files"
	"github.com/zeebo/errs"
)

var invalidUsage = errs.Class("invalid usage")

var (
	filesConfigPath = flag.String("files_config", "files.json",
		"path to json file containing the config for the files backend")
)

func main() {
	flag.Parse()

	err := run(context.Background(), flag.Args())
	if err == nil {
		return
	}

	switch {
	case invalidUsage.Has(err):
		fmt.Fprintf(os.Stderr, "%v\n\n", err)
		printUsage(os.Stderr)

	default:
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}

	os.Exit(1)
}

func run(ctx context.Context, args []string) (err error) {
	if len(args) == 0 {
		return invalidUsage.New("no command specified")
	}

	files_config_data, err := ioutil.ReadFile(*filesConfigPath)
	if err != nil {
		return errs.Wrap(err)
	}

	var filesOptions struct {
		files.Options
		Dir string
	}

	if err := json.Unmarshal(files_config_data, &filesOptions); err != nil {
		return errs.Wrap(err)
	}

	di := files.New(filesOptions.Dir, filesOptions.Options)

	switch cmd := args[0]; cmd {
	case "latest":
		if len(args) == 1 {
			return errs.New("no metric specified")
		}

		return runLatest(ctx, di, args[1])

	case "render":
		if len(args) == 1 {
			return errs.New("no metric specified")
		}
		if len(args) == 2 {
			return errs.New("no duration specified")
		}

		dur, err := time.ParseDuration(args[2])
		if err != nil {
			return errs.Wrap(err)
		}

		return runRender(ctx, di, args[1], dur)

	case "serve":
		if len(args) == 1 {
			return errs.New("no address specified")
		}

		go func() {
			n := time.Now()
			fmt.Println("populating metrics...")
			fmt.Println("done. err:", di.PopulateMetrics(ctx), time.Since(n))
		}()

		return errs.Wrap(http.ListenAndServe(args[1], api.New(di)))

	default:
		return invalidUsage.New("unknown command: %q", args[0])
	}

	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`
usage: query <command> [args...]

command can be one of:

	latest   query the latest value for the metric specified in args
	query    save out a default image for some metric as test.png
	serve    run an http server for serving requests
`))
}
