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
	fmt.Fprintln(w, greeting())
}

func greeting() string {
	return "Hello, gitstreams!"
}
