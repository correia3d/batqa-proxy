// BATQA Proxy - Proxy TCP transparente para TeamSpeak/TeaSpeak ServerQuery
//
// Acelera comandos executando-os localmente no servidor, eliminando
// latência de rede entre o proxy e o TeamSpeak.
//
// Uso: ./batqa-proxy -listen :10202 -target localhost:10011
//
// Build: go build -o batqa-proxy main.go
// Build Linux (cross-compile): GOOS=linux GOARCH=amd64 go build -o batqa-proxy-linux-amd64 main.go

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Configuração do proxy
type Config struct {
	ListenAddr    string
	TargetAddr    string
	MaxConns      int
	Timeout       time.Duration
	LogLevel      string
}

// Estatísticas do proxy
type Stats struct {
	TotalConnections   uint64
	ActiveConnections  int64
	TotalCommands      uint64
	TotalBytes         uint64
	StartTime          time.Time
}

// Proxy principal
type Proxy struct {
	config      Config
	stats       Stats
	listener    net.Listener
	shutdown    chan struct{}
	wg          sync.WaitGroup
}

func NewProxy(config Config) *Proxy {
	return &Proxy{
		config:      config,
		stats:       Stats{StartTime: time.Now()},
		shutdown:    make(chan struct{}),
	}
}

func (p *Proxy) Start() error {
	listener, err := net.Listen("tcp", p.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("erro ao iniciar listener: %w", err)
	}
	p.listener = listener

	log.Printf("🚀 BATQA Proxy iniciado")
	log.Printf("   Escutando em: %s", p.config.ListenAddr)
	log.Printf("   Destino: %s", p.config.TargetAddr)
	log.Printf("   Max conexões: %d", p.config.MaxConns)
	log.Printf("   Rate limit: unlimited")

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-p.shutdown:
				return nil
			default:
				log.Printf("Erro ao aceitar conexão: %v", err)
				continue
			}
		}

		// Verifica limite de conexões
		if atomic.LoadInt64(&p.stats.ActiveConnections) >= int64(p.config.MaxConns) {
			log.Printf("⚠️  Limite de conexões atingido, rejeitando: %s", conn.RemoteAddr())
			conn.Close()
			continue
		}

		p.wg.Add(1)
		go p.handleConnection(conn)
	}
}

func (p *Proxy) Stop() {
	close(p.shutdown)
	if p.listener != nil {
		p.listener.Close()
	}
	p.wg.Wait()
	log.Printf("✅ Proxy encerrado")
}

func (p *Proxy) handleConnection(clientConn net.Conn) {
	defer p.wg.Done()
	defer clientConn.Close()

	atomic.AddUint64(&p.stats.TotalConnections, 1)
	atomic.AddInt64(&p.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&p.stats.ActiveConnections, -1)

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("📥 Nova conexão: %s (ativas: %d)", clientAddr, atomic.LoadInt64(&p.stats.ActiveConnections))

	// Conecta no TeamSpeak local
	tsConn, err := net.DialTimeout("tcp", p.config.TargetAddr, p.config.Timeout)
	if err != nil {
		log.Printf("❌ Erro ao conectar no TS: %v", err)
		return
	}
	defer tsConn.Close()

	// Define timeouts
	clientConn.SetDeadline(time.Time{}) // Sem deadline global
	tsConn.SetDeadline(time.Time{})

	// Contador de bytes/comandos para esta conexão
	var bytesTransferred uint64
	var commandCount uint64

	// Pipe bidirecional
	done := make(chan struct{}, 2)

	// Cliente → TeamSpeak (conta comandos)
	go func() {
		reader := bufio.NewReader(clientConn)
		writer := bufio.NewWriter(tsConn)
		
		for {
			// Lê linha do cliente
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Erro leitura cliente: %v", err)
				}
				break
			}

			// Envia pro TS
			_, err = writer.Write(line)
			if err != nil {
				log.Printf("Erro escrita TS: %v", err)
				break
			}
			writer.Flush()

			bytesTransferred += uint64(len(line))
			commandCount++
			atomic.AddUint64(&p.stats.TotalCommands, 1)
			atomic.AddUint64(&p.stats.TotalBytes, uint64(len(line)))
		}
		done <- struct{}{}
	}()

	// TeamSpeak → Cliente
	go func() {
		reader := bufio.NewReader(tsConn)
		writer := bufio.NewWriter(clientConn)

		for {
			// Lê resposta do TS
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Erro leitura TS: %v", err)
				}
				break
			}

			// Envia pro cliente
			_, err = writer.Write(line)
			if err != nil {
				log.Printf("Erro escrita cliente: %v", err)
				break
			}
			writer.Flush()

			bytesTransferred += uint64(len(line))
			atomic.AddUint64(&p.stats.TotalBytes, uint64(len(line)))
		}
		done <- struct{}{}
	}()

	// Espera uma das direções terminar
	<-done

	log.Printf("📤 Conexão encerrada: %s (comandos: %d, bytes: %d)", 
		clientAddr, commandCount, bytesTransferred)
}

func (p *Proxy) PrintStats() {
	uptime := time.Since(p.stats.StartTime)
	log.Printf("📊 Estatísticas:")
	log.Printf("   Uptime: %s", uptime.Round(time.Second))
	log.Printf("   Total conexões: %d", atomic.LoadUint64(&p.stats.TotalConnections))
	log.Printf("   Conexões ativas: %d", atomic.LoadInt64(&p.stats.ActiveConnections))
	log.Printf("   Total comandos: %d", atomic.LoadUint64(&p.stats.TotalCommands))
	log.Printf("   Total bytes: %d", atomic.LoadUint64(&p.stats.TotalBytes))
}

func main() {
	// Flags de linha de comando
	listenAddr := flag.String("listen", ":10202", "Endereço para escutar (ex: :10202)")
	targetAddr := flag.String("target", "localhost:10011", "Endereço do TeamSpeak ServerQuery")
	maxConns := flag.Int("max-conns", 100, "Máximo de conexões simultâneas")
	timeout := flag.Duration("timeout", 30*time.Second, "Timeout de conexão")
	logLevel := flag.String("log", "info", "Nível de log (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Mostra versão e sai")

	flag.Parse()

	if *showVersion {
		fmt.Println("BATQA Proxy v1.0.0")
		fmt.Println("Proxy TCP para TeamSpeak/TeaSpeak ServerQuery")
		os.Exit(0)
	}

	// Configura log
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("[BATQA-Proxy] ")

	config := Config{
		ListenAddr: *listenAddr,
		TargetAddr: *targetAddr,
		MaxConns:   *maxConns,
		Timeout:    *timeout,
		LogLevel:   *logLevel,
	}

	proxy := NewProxy(config)

	// Captura sinais para shutdown gracioso
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n⏹️  Recebido sinal de shutdown...")
		proxy.PrintStats()
		proxy.Stop()
		os.Exit(0)
	}()

	// Imprime estatísticas periodicamente
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			proxy.PrintStats()
		}
	}()

	// Inicia proxy
	if err := proxy.Start(); err != nil {
		log.Fatalf("Erro fatal: %v", err)
	}
}
