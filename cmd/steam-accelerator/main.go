package main

import (
	"flag"
	"fmt"
	"os"

	steamcore "github.com/gofurry/go-steam-core"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s (%s)\n", steamcore.ProjectName, steamcore.Version, steamcore.ModulePath)
		return
	}

	fmt.Fprintf(os.Stdout, "%s repository scaffold\n", steamcore.ProjectName)
	fmt.Fprintln(os.Stdout, "Runtime acceleration commands will be added in v0.1.0.")
}
