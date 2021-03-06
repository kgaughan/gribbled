package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var Version string

var root string
var hostname string
var port string

func init() {
	defaultHostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&root, "root", "/srv/gopher", "Root directory of server")
	flag.StringVar(&hostname, "hostname", defaultHostname, "Hostname to present")
	flag.StringVar(&port, "port", "70", "Port to bind to")
}

func main() {
	flag.Parse()

	if _, err := os.Stat(root); os.IsNotExist(err) {
		log.Fatalf("Root directory '%v' not found", root)
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(hostname, port))
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		if conn, err := ln.Accept(); err != nil {
			log.Print(err)
		} else {
			go handleConnection(conn)
		}
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		// Format is <selector>TAB<query>CRLF
		parts := strings.SplitN(strings.TrimRight(scanner.Text(), "\r\n"), "\t", 2)

		// Make sure the selector is safe
		parts[0] = path.Clean(parts[0])
		if parts[0] == "." {
			parts[0] = ""
		}
		if strings.HasPrefix(parts[0], "..") {
			writeError(conn, "Bad selector")
			return
		}

		localPath := filepath.Join(root, parts[0])
		if err := resolve(conn, localPath, parts[0]); err != nil {
			log.Print(err)
			writeError(conn, err.Error())
		}
	} else if err := scanner.Err(); err != nil {
		log.Print(err)
	}
}

func resolve(out io.Writer, localPath string, selector string) error {
	if fi, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("Cannot find %s", selector)
	} else if fi.IsDir() {
		gophermap := filepath.Join(localPath, "gophermap")
		if _, err := os.Stat(gophermap); err == nil {
			err := loadGopherMap(out, localPath, selector)
			out.Write([]byte(".\r\n"))
			return err
		}
		if catalogue, err := listDirectory(localPath, selector); err != nil {
			return err
		} else {
			write(out, catalogue)
			return nil
		}
	}

	return sendFile(out, localPath)
}

func sendFile(out io.Writer, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(out, f); err != nil {
		return err
	}
	return nil
}
