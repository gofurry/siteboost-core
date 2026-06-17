package main

import (
	"fmt"

	steamcore "github.com/gofurry/go-steam-core"
)

func main() {
	fmt.Printf("%s is available as %s\n", steamcore.ProjectName, steamcore.ModulePath)
}
