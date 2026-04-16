package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

type config struct {
	ServerAddr string
	Visitors   []visitor
}

type visitor struct {
	Name       string
	ServerName string
	SecretKey  string
	BindAddr   string
	BindPort   int
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing args")
		os.Exit(1)
	}

	switch args[0] {
	case "verify":
		runVerify(args[1:])
	default:
		runClient(args)
	}
}

func runVerify(args []string) {
	configPath := parseConfigPath(args)
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.ServerAddr == "verify-fail.local" {
		fmt.Fprintln(os.Stderr, "syntax error near visitors")
		os.Exit(1)
	}
	fmt.Println("frpc: the configuration file syntax is ok")
}

func runClient(args []string) {
	configPath := parseConfigPath(args)
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch cfg.ServerAddr {
	case "auth-fail.local":
		fmt.Fprintln(os.Stderr, "auth token invalid")
		os.Exit(1)
	case "exit-now.local":
		fmt.Fprintln(os.Stderr, "frpc exited immediately")
		os.Exit(1)
	}

	listeners := make([]net.Listener, 0, len(cfg.Visitors))
	for _, v := range cfg.Visitors {
		ln, err := net.Listen("tcp", net.JoinHostPort(v.BindAddr, strconv.Itoa(v.BindPort)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "address already in use")
			os.Exit(1)
		}
		listeners = append(listeners, ln)
		go serve(ln, v)
	}

	fmt.Println("frpc started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	for _, ln := range listeners {
		_ = ln.Close()
	}
}

func serve(ln net.Listener, v visitor) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
		if strings.Contains(v.ServerName, "missing-service") {
			fmt.Fprintln(os.Stderr, "proxy name doesn't exist")
		}
		if v.SecretKey == "bad-secret" {
			fmt.Fprintln(os.Stderr, "secret key mismatch")
		}
	}
}

func parseConfigPath(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		if args[idx] == "-c" && idx+1 < len(args) {
			return args[idx+1]
		}
	}
	fmt.Fprintln(os.Stderr, "missing -c <config>")
	os.Exit(1)
	return ""
}

func loadConfig(path string) (config, error) {
	file, err := os.Open(path)
	if err != nil {
		return config{}, err
	}
	defer file.Close()

	var cfg config
	var current *visitor

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[visitors]]" {
			cfg.Visitors = append(cfg.Visitors, visitor{BindAddr: "127.0.0.1"})
			current = &cfg.Visitors[len(cfg.Visitors)-1]
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])
		value := strings.Trim(raw, "\"")

		switch key {
		case "serverAddr":
			cfg.ServerAddr = value
		case "name":
			if current != nil {
				current.Name = value
			}
		case "serverName":
			if current != nil {
				current.ServerName = value
			}
		case "secretKey":
			if current != nil {
				current.SecretKey = value
			}
		case "bindAddr":
			if current != nil {
				current.BindAddr = value
			}
		case "bindPort":
			if current != nil {
				port, _ := strconv.Atoi(value)
				current.BindPort = port
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return config{}, err
	}
	return cfg, nil
}
