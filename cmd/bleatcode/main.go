package main

import (
	"flag"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"

	"github.com/penta/BleatCode/internal/agent"
	"github.com/penta/BleatCode/internal/config"
	"github.com/penta/BleatCode/internal/tui"
)

func main() {
	configPath := flag.String("c", config.DefaultPath(), "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	sel, err := cfg.SelectedModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	clientCfg := openai.DefaultConfig(sel.APIKey)
	if sel.BaseURL != "" {
		clientCfg.BaseURL = sel.BaseURL
	}
	client := openai.NewClientWithConfig(clientCfg)

	workDir, _ := os.Getwd()
	loop := agent.NewLoop(client, sel.ModelID, agent.LoopConfig{
		WorkDir: workDir,
	})

	if err := tui.Run(loop, sel.ModelID, workDir); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
