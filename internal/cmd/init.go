package cmd

import (
	"fmt"
	"os"
)

func init() {
	if os.Getenv("GOD_HUNTER_DEBUG") == "1" {
		fmt.Println("[DEBUG] God-Hunter initializing...")
	}
}
