package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// version é preenchida no build de release (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	dataDir := flag.String("data", "data", "pasta onde ficam config.json e a fila")
	port := flag.Int("port", 0, "porta do painel (padrão: a do config.json, 7777)")
	noBrowser := flag.Bool("no-browser", false, "não abrir o navegador automaticamente")
	showVersion := flag.Bool("version", false, "mostra a versão e sai")
	flag.Parse()
	if *showVersion {
		fmt.Println("hero-requests", version)
		return
	}

	app, err := NewApp(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	p := app.Config().Port
	if *port != 0 {
		p = *port
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		log.Fatalf("não consegui abrir a porta %d (já tem outro programa usando?): %v\nUse -port para escolher outra.", p, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app.Run(ctx)

	srv := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sc)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d", p)
	fmt.Printf("Hero Requests %s\n  Painel:  %s\n  Overlay: %s/overlay  (adicione no OBS como Browser Source)\n  Dados:   %s\n\nFeche esta janela para encerrar.\n", version, url, url, *dataDir)
	if !*noBrowser {
		go func() { time.Sleep(300 * time.Millisecond); openBrowser(url) }()
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
