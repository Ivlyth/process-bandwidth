package main

import (
	"github.com/Ivlyth/process-bandwidth/commands"
	"github.com/Ivlyth/process-bandwidth/logging"
)

var logger = logging.GetLogger()

func main() {
	err := commands.Execute()
	if err != nil {
		logger.Fatalf("error when execute commands: %s", err)
	}
}
