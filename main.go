// secure-compose: Docker Compose with age-encrypted secrets
package main

import (
	"os"

	"github.com/rafitox/secure-compose/internal/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
}
