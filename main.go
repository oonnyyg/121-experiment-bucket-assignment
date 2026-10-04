package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "", "path to experiment config JSON")
	usersPath := flag.String("users", "", "path to users CSV (first column user_id)")
	decisionsPath := flag.String("decisions", "decisions.jsonl", "output JSONL decisions path")
	summaryPath := flag.String("summary", "summary.csv", "output summary CSV path")
	flag.Parse()
	if *configPath == "" || *usersPath == "" {
		fmt.Fprintln(os.Stderr, "usage: bucketassign -config config.json -users users.csv [-decisions out.jsonl] [-summary out.csv]")
		os.Exit(2)
	}
	if err := run(*configPath, *usersPath, *decisionsPath, *summaryPath); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(configPath, usersPath, decisionsPath, summaryPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	users, err := ReadUsers(usersPath)
	if err != nil {
		return err
	}
	if err := Run(cfg, users, decisionsPath, summaryPath); err != nil {
		os.Remove(decisionsPath)
		os.Remove(summaryPath)
		return err
	}
	return nil
}
