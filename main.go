package main

import (
	"os"

	"github.com/Hayao0819/hytop/internal/cmd"
	"github.com/Hayao0819/hytop/internal/version"
)

func main() { os.Exit(cmd.Execute(version.Current().Version)) }
