package main

import (
	"github.com/bnkamalesh/phispr/cmd/http"
	"github.com/bnkamalesh/phispr/internal/configs"
	"github.com/bnkamalesh/phispr/internal/pkg/sysignals"
)

func main() {
	fatalChan := make(chan error)
	go sysignals.NotifyErrorOnQuit(fatalChan)
	cfg := configs.Load("./config.yaml")
	onExit := http.Start(cfg)
	<-fatalChan
	onExit()
}
