package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/nds-stack/commandcode-go-proxy/internal/proxy"
	"github.com/nds-stack/commandcode-go-proxy/internal/server"
	"github.com/nds-stack/commandcode-go-proxy/internal/update"
)

const appVersion = "v0.2.1"
const repositoryURL = "https://github.com/nds-stack/commandcode-go-proxy"
const debugLogging = false

func main() {
	port := flag.String("port", "", "Port to run the server on (default: 9173)")
	host := flag.String("host", "", "Host to bind to (default: 127.0.0.1)")
	apiKey := flag.String("api-key", "", "API key for CommandCode (optional, can also be set via Authorization header)")
	timeout := flag.Duration("timeout", 600*time.Second, "Request timeout (e.g. 600s or 10m)")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionText())
		return
	}

	proxy := proxy.NewProxy(*apiKey, *timeout)
	proxy.Debug = debugLogging || *debugFlag

	srv := server.NewServer(proxy)
	srv.SetPort(*port)
	srv.SetHost(*host)

	printStartupInfo(srv)

	srv.Start()
}

func versionText() string {
	latest, hasUpdate, err := update.LatestVersion(appVersion)
	if err != nil || !hasUpdate {
		return appVersion
	}
	return fmt.Sprintf("%s (latest: %s)", appVersion, latest)
}

func printStartupInfo(srv *server.Server) {
	fmt.Println("")
	fmt.Println("========================================")
	fmt.Println("  CommandCode Proxy Server")
	fmt.Println("========================================")
	fmt.Println("")
	fmt.Printf("  Version:     %s\n", versionText())
	fmt.Printf("  Repository:  %s\n", repositoryURL)
	fmt.Printf("  Host:        %s\n", srv.GetHost())
	fmt.Printf("  Port:        %s\n", srv.GetPort())
	fmt.Println("  Base URL:    https://api.commandcode.ai")
	fmt.Println("")
	fmt.Println("  Endpoints:")
	fmt.Println("    POST /v1/chat/completions  (OpenAI-compatible)")
	fmt.Println("    POST /chat/completions     (OpenAI-compatible alias)")
	fmt.Println("    POST /v1/responses         (OpenAI Responses-compatible)")
	fmt.Println("    GET  /v1/models            (list models)")
	fmt.Println("    GET  /health               (health check)")
	fmt.Println("")
	fmt.Printf("  Server running on http://%s:%s\n", srv.GetHost(), srv.GetPort())
	fmt.Println("")
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println("========================================")
}
