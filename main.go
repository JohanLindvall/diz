package main

import (
	"compress/flate"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/JohanLindvall/diz/app"
	"github.com/docker/docker/client"
)

const usage = "usage: diz [flags] <list|create|update|restore|serve> [args...]"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var opts app.Options
	flag.StringVar(&opts.FromZip, "fromzip", "", "Set to read Docker tags and images from zip file")
	flag.StringVar(&opts.TagFile, "tagfile", "", "Set to read and write tags from file")
	flag.BoolVar(&opts.DigestTags, "digest", false, "If set, update tags to use repo digest")
	flag.BoolVar(&opts.Pull, "pull", false, "If set, pulls images from docker registry")
	flag.IntVar(&opts.Level, "level", flate.DefaultCompression, "Sets the deflate compression level (0-9)")
	flag.StringVar(&opts.RegistryAddress, "registryAddress", "", "Sets the registry address of the given docker references")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		return errors.New(usage)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	a := app.New(cli, opts)
	cmd, rest := args[0], args[1:]

	switch cmd {
	case "list":
		return a.List(a.Tags(rest))
	case "create":
		if len(rest) < 1 {
			return errors.New("usage: diz create <file> [tags...]")
		}
		return a.Create(rest[0], a.Tags(rest[1:]))
	case "update":
		if len(rest) < 2 {
			return errors.New("usage: diz update <initial> <file> [tags...]")
		}
		return a.Update(rest[0], rest[1], a.Tags(rest[2:]))
	case "restore":
		return a.Restore(a.Tags(rest))
	case "serve":
		if len(rest) < 1 {
			return errors.New("usage: diz serve <file>")
		}
		return a.Serve(rest[0])
	default:
		return fmt.Errorf("unknown command %q\n%s", cmd, usage)
	}
}
