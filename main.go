package main

import "github.com/MunifTanjim/pushport/internal/cli"

// version is overridden at build time via -ldflags.
var version = "dev"

func main() { cli.Execute(version) }
