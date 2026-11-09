package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if runtime.GOOS != "darwin" {
		fmt.Fprintf(os.Stderr, "Error: defaults2nix needs macOS, it reads preferences with the `defaults` command (this is %s).\n", runtime.GOOS)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, defaultsSource{}))
}

func run(args []string, stdout, stderr io.Writer, src source) int {
	fs := flag.NewFlagSet("defaults2nix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}

	all := fs.Bool("all", false, "convert every domain that defaults read prints")
	split := fs.Bool("split", false, "write one Nix file per domain")
	out := fs.String("out", "", "output file, or directory with -split")
	fs.StringVar(out, "o", "", "same as -out")
	filterFlag := fs.String("filter", "", "comma-separated values to drop: dates,state,uuids")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(fs, stdout)
			return 0
		}
		usage(fs, stderr)
		return 2
	}

	switch {
	case *all && *split:
		fmt.Fprintln(stderr, "Error: -all and -split cannot be combined.")
		return 2
	case (*all || *split) && fs.NArg() > 0:
		fmt.Fprintln(stderr, "Error: -all and -split take no domain.")
		return 2
	case *split && *out == "":
		fmt.Fprintln(stderr, "Error: -out is required with -split.")
		return 2
	case fs.NArg() > 1:
		fmt.Fprintf(stderr, "Error: unexpected argument %q; one domain only, flags first.\n", fs.Arg(1))
		return 2
	}

	filter, err := parseFilter(*filterFlag)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	if !*split && *out != "" {
		if info, err := os.Stat(*out); err == nil && info.IsDir() {
			fmt.Fprintf(stderr, "Error: %s is a directory, -out takes a file unless you also pass -split.\n", *out)
			return 2
		}
	}

	switch {
	case *split:
		return writeDomains(*out, src, filter, stderr)
	case *all:
		text, err := src.All()
		if err != nil {
			return fail(stderr, err)
		}
		return output(text, *out, filter, true, stdout, stderr)
	case fs.NArg() > 0:
		text, err := src.Read(fs.Arg(0))
		if err != nil {
			return fail(stderr, err)
		}
		return output(text, *out, filter, false, stdout, stderr)
	}

	usage(fs, stderr)
	return 2
}

// convert renders one `defaults read` document as Nix.
func convert(text string, f Filter) (string, error) {
	v, err := parse(text)
	if err != nil {
		return "", err
	}
	if f.drop("", v) {
		v = dict{}
	}
	return f.apply(v).nix(0), nil
}

// convertAll renders the domain map that `-all` reads.
func convertAll(text string, f Filter) (string, error) {
	v, err := parse(text)
	if err != nil {
		return "", err
	}
	return f.applyDomains(v).nix(0), nil
}

// output converts text and writes the Nix to out, or to stdout when out is empty.
func output(text, out string, f Filter, domains bool, stdout, stderr io.Writer) int {
	render := convert
	if domains {
		render = convertAll
	}
	nix, err := render(text, f)
	if err != nil {
		return fail(stderr, err)
	}
	if out == "" {
		fmt.Fprintln(stdout, nix)
		return 0
	}
	if err := os.WriteFile(out, []byte(nix+"\n"), 0o644); err != nil {
		return fail(stderr, err)
	}
	return 0
}

// writeDomains writes one Nix file per domain into dir.
func writeDomains(dir string, src source, f Filter, stderr io.Writer) int {
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		fmt.Fprintf(stderr, "Error: %s is not a directory, -split needs one.\n", dir)
		return 2
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(stderr, err)
	}
	domains, err := src.Domains()
	if err != nil {
		return fail(stderr, err)
	}

	var written int
	var skipped, failed, unwritten []string
	byFile := map[string]string{}
	for _, domain := range domains {
		text, err := src.Read(domain)
		if err != nil {
			failed = append(failed, domain)
			continue
		}
		nix, err := convert(text, f)
		if err != nil {
			failed = append(failed, domain)
			continue
		}
		if strings.TrimSpace(nix) == "{}" {
			skipped = append(skipped, domain)
			continue
		}
		name := sanitizeFilename(domain) + ".nix"
		if other, taken := byFile[name]; taken {
			fmt.Fprintf(stderr, "Warning: %s and %s both write %s; keeping %s.\n", other, domain, name, other)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(nix+"\n"), 0o644); err != nil {
			fmt.Fprintf(stderr, "Warning: %v\n", err)
			unwritten = append(unwritten, domain)
			continue
		}
		byFile[name] = domain
		written++
	}

	if len(skipped) > 0 {
		fmt.Fprintf(stderr, "Info: skipped %d empty domains.\n", len(skipped))
	}
	if len(failed) > 0 {
		fmt.Fprintf(stderr, "Warning: %d domains failed to read or parse: %s\n", len(failed), strings.Join(failed, ", "))
	}
	if len(unwritten) > 0 {
		fmt.Fprintf(stderr, "Warning: %d domains could not be written: %s\n", len(unwritten), strings.Join(unwritten, ", "))
	}
	if written == 0 {
		fmt.Fprintf(stderr, "Error: nothing was written to %s.\n", dir)
		return 1
	}
	fmt.Fprintf(stderr, "Wrote %d domains to %s\n", written, dir)
	return 0
}

func parseFilter(names string) (Filter, error) {
	var f Filter
	for _, name := range strings.Split(names, ",") {
		switch strings.TrimSpace(strings.ToLower(name)) {
		case "":
		case "dates":
			f.Dates = true
		case "state":
			f.State = true
		case "uuids":
			f.UUIDs = true
		default:
			return Filter{}, fmt.Errorf("unknown filter %q (want dates, state or uuids)", name)
		}
	}
	return f, nil
}

// sanitizeFilename turns a domain into a file name.
func sanitizeFilename(domain string) string {
	name := strings.ReplaceAll(domain, ".", "-")
	name = strings.ReplaceAll(name, " ", "_")
	return strings.ReplaceAll(name, "/", "_")
}

func usage(fs *flag.FlagSet, w io.Writer) {
	fs.SetOutput(w)
	fmt.Fprintf(w, "Usage: %s [flags] [domain]\n\n", fs.Name())
	fmt.Fprintln(w, "Convert macOS `defaults` output to Nix.")
	fmt.Fprintln(w, "\nFlags:")
	fs.PrintDefaults()
	fmt.Fprintln(w, "\nExamples:")
	fmt.Fprintln(w, "  defaults2nix com.apple.Safari")
	fmt.Fprintln(w, "  defaults2nix -out safari.nix com.apple.Safari")
	fmt.Fprintln(w, "  defaults2nix -all -out all.nix")
	fmt.Fprintln(w, "  defaults2nix -all -filter dates,state,uuids -out clean.nix")
	fmt.Fprintln(w, "  defaults2nix -split -out ./configs/")
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "Error: %v\n", err)
	return 1
}
