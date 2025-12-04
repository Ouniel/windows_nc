package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

// Config 存储配置信息
type Config struct {
	Port      string
	Protocol  string // "tcp", "udp", "tls"
	LogFile   string // 记录会话到文件
	Verbose   bool   // 显示详细的数据流日志
	SmartMode bool   // 智能处理换行符 (Windows -> Linux)
}

func main() {
	// 定义命令行参数
	port := flag.String("p", "4444", "Local port to listen on")
	udpMode := flag.Bool("u", false, "Listen in UDP mode (e.g., nc -u, /dev/udp)")
	tlsMode := flag.Bool("tls", false, "Listen in TLS/SSL mode (e.g., openssl s_client)")
	logFile := flag.String("log", "", "File to log the session output")
	verbose := flag.Bool("v", false, "Verbose mode (show byte count)")
	smart := flag.Bool("smart", true, "Smart newline mode (Strip \\r for Linux shells)")
	help := flag.Bool("h", false, "Show help message")
	flag.Parse()

	if *help {
		printUsage()
		return
	}

	config := Config{
		Port:      *port,
		LogFile:   *logFile,
		Verbose:   *verbose,
		SmartMode: *smart,
	}

	// 确定协议模式
	if *udpMode {
		config.Protocol = "udp"
	} else if *tlsMode {
		config.Protocol = "tls"
	} else {
		config.Protocol = "tcp"
	}

	fmt.Printf("[*] Starting SimpleNC Listener...\n")
	fmt.Printf("[*] Mode: %s | Port: %s | Smart Newlines: %v\n", config.Protocol, config.Port, config.SmartMode)
	if config.LogFile != "" {
		fmt.Printf("[*] Logging session to: %s\n", config.LogFile)
	}

	// 根据模式启动监听
	switch config.Protocol {
	case "udp":
		startUDPListener(config)
	case "tls":
		startTLSListener(config)
	default:
		startTCPListener(config)
	}
}

// startTCPListener 启动标准 TCP 监听
func startTCPListener(cfg Config) {
	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[-] Error binding to port: %v", err)
	}
	defer listener.Close()

	fmt.Printf("[*] Listening on %s (TCP)...\n", addr)
	acceptLoop(listener, cfg)
}

// startTLSListener 启动加密的 TLS 监听
func startTLSListener(cfg Config) {
	fmt.Println("[*] Generating ephemeral self-signed certificate for TLS...")
	cert, key, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("[-] Failed to generate cert: %v", err)
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		log.Fatalf("[-] Failed to load cert key pair: %v", err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)

	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		log.Fatalf("[-] Error binding to port: %v", err)
	}
	defer listener.Close()

	fmt.Printf("[*] Listening on %s (TLS/SSL)...\n", addr)
	acceptLoop(listener, cfg)
}

// acceptLoop 处理 TCP/TLS 连接
func acceptLoop(listener net.Listener, cfg Config) {
	conn, err := listener.Accept()
	if err != nil {
		log.Fatalf("[-] Failed to accept connection: %v", err)
	}
	defer conn.Close()

	// 关键优化：针对 TCP 连接禁用 Nagle 算法，确保小包（命令回显）立即发送
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
	}

	fmt.Printf("\n[+] Connection received from %s\n", conn.RemoteAddr().String())
	fmt.Println("[!] Shell session started. Press Ctrl+C to exit.")
	fmt.Println("---------------------------------------------------")

	handleStream(conn, conn, cfg)
}

// startUDPListener 启动 UDP 监听
func startUDPListener(cfg Config) {
	addrStr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		log.Fatalf("[-] Resolve error: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("[-] Error binding to port: %v", err)
	}
	defer conn.Close()

	fmt.Printf("[*] Listening on %s (UDP)...\n", addrStr)
	fmt.Println("[*] Waiting for first packet to lock session...")

	buf := make([]byte, 65535)

	n, remoteAddr, err := conn.ReadFromUDP(buf)
	if err != nil {
		log.Fatalf("[-] Read error: %v", err)
	}

	fmt.Printf("\n[+] UDP Packet received from %s\n", remoteAddr.String())
	fmt.Printf("[+] Locked session to %s\n", remoteAddr.String())
	fmt.Println("---------------------------------------------------")

	var outputWriter io.Writer = os.Stdout
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[-] Warning: Could not open log file: %v", err)
		} else {
			defer f.Close()
			outputWriter = io.MultiWriter(os.Stdout, f)
		}
	}

	outputWriter.Write(buf[:n])

	done := make(chan struct{})

	// UDP Read Loop
	go func() {
		defer func() { done <- struct{}{} }()
		readBuf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFromUDP(readBuf)
			if err != nil {
				return
			}
			if addr.String() == remoteAddr.String() {
				if cfg.Verbose {
					fmt.Printf("\r[DEBUG] Read %d bytes from UDP\n", n)
				}
				outputWriter.Write(readBuf[:n])
			}
		}
	}()

	// UDP Write Loop
	go func() {
		inputBuf := make([]byte, 65507)
		for {
			n, err := os.Stdin.Read(inputBuf)
			if err != nil {
				return
			}
			if n > 0 {
				dataToSend := inputBuf[:n]
				// 智能换行处理：移除 Windows 的 \r
				if cfg.SmartMode {
					dataToSend = bytes.ReplaceAll(dataToSend, []byte{'\r'}, []byte{})
				}
				conn.WriteToUDP(dataToSend, remoteAddr)
			}
		}
	}()

	<-done
	fmt.Println("\n[*] Connection closed.")
}

// handleStream 处理双向数据拷贝 (核心优化部分)
func handleStream(reader io.Reader, writer io.Writer, cfg Config) {
	// 准备日志输出
	var outputWriter io.Writer = os.Stdout
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[-] Warning: Could not open log file: %v", err)
		} else {
			defer f.Close()
			outputWriter = io.MultiWriter(os.Stdout, f)
		}
	}

	done := make(chan struct{})

	// 1. 远程 -> 本地 (显式循环，确保不缓冲)
	go func() {
		defer func() { done <- struct{}{} }()
		// 使用较小的 Buffer 确保即使是小的回显也能立即感知
		buf := make([]byte, 32*1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				if cfg.Verbose {
					log.Printf("[DEBUG] Recv %d bytes", n)
				}
				// 立即写入 Stdout
				_, wErr := outputWriter.Write(buf[:n])
				if wErr != nil {
					log.Printf("[-] Output write error: %v", wErr)
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Println("\n[*] Remote connection lost.")
				}
				return
			}
		}
	}()

	// 2. 本地 -> 远程 (处理键盘输入)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				dataToSend := buf[:n]

				// 智能模式：如果是在 Windows 上运行，剔除 \r，
				// 这样 Linux 收到的是纯净的 \n，避免 'command not found' 错误
				if cfg.SmartMode {
					dataToSend = bytes.ReplaceAll(dataToSend, []byte{'\r'}, []byte{})
				}

				if cfg.Verbose {
					log.Printf("[DEBUG] Sent %d bytes", len(dataToSend))
				}

				_, wErr := writer.Write(dataToSend)
				if wErr != nil {
					log.Printf("[-] Socket write error: %v", wErr)
					return
				}
			}
			if err != nil {
				return // Stdin closed
			}
		}
	}()

	<-done
}

// generateSelfSignedCert 生成证书 (保持不变)
func generateSelfSignedCert() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         false,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}), nil
}

func printUsage() {
	fmt.Println(`NC - Enhanced Go Netcat

Usage: go run nc.go [flags]

Flags:
  -p <port>     Port to listen on (default: 4444)
  -u            UDP Mode
  -tls          TLS/SSL Mode
  -smart        Smart Newlines (Strip \r for Linux targets) (default: true)
  -v            Verbose (debug data flow)
  -log <file>   Log output to file`)
}
