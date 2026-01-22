package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	run(os.Stdout)
}

func run(w io.Writer) {
	_, _ = fmt.Fprintln(w, greeting())
}

func greeting() string {
	return "Hello, gitstreams!"
}
