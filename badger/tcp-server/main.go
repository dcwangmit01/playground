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
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/badger"
)

type operation struct {
	Operator string
	Params   []string
}

type operationHandler func(*bufio.Writer, *badger.KV, ...string) error

var operationMap = map[string]operationHandler{
	"GET":    handleGet,
	"SET":    handleSet,
	"DELETE": handleDelete,
	"KEYS":   handleKeys,
	"QUIT":   handleQuit,
	"":       handleEmpty,
}

var isServing = true
var wg sync.WaitGroup

func main() {
	path := flag.String("path", "/var/lib/badger", "path to KV store")
	address := flag.String("address", ":36379", "address to bind to")
	syncWrite := flag.Bool("sync", false, "sync every write")

	flag.Parse()

	// Make sure the KV store path is a directory
	if fileInfo, err := os.Stat(*path); err == nil {
		// path exists
		if !fileInfo.Mode().IsDir() {
			log.Fatalf("[%s] is not a directory\n", *path)
		}
	} else {
		// path does not exist
		if err := os.MkdirAll(*path, 0755); err != nil {
			log.Fatalf("unable to create [%s]: %v\n", *path, err)
		}
	}

	// open KV store
	opt := badger.DefaultOptions
	opt.Dir = *path
	opt.SyncWrites = *syncWrite
	opt.ValueLogFileSize = 1 << 28
	opt.ValueGCRunInterval = 1 * time.Minute
	kv, err := badger.NewKV(&opt)
	if err != nil {
		log.Fatalf("unable to open KV store [%s]: %v\n", *path, err)
	}
	defer kv.Close()

	// shutdown gracefully when SIGTERM/SIGINT
	chanSignal := make(chan os.Signal)
	signal.Notify(chanSignal, os.Interrupt, syscall.SIGTERM)
	signal.Notify(chanSignal, os.Interrupt, syscall.SIGINT)

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("Failed to listen on [%s]: %v\n", *address, err)
	}
	defer listener.Close()

	// handle incoming connection requests
	chanConnection := make(chan net.Conn)
	go func() {
		for isServing {
			if conn, err := listener.Accept(); err != nil && isServing {
				log.Printf("Failed to accept: %v\n", err)
			} else {
				chanConnection <- conn
			}
		}
	}()

	for isServing {
		select {
		case sig := <-chanSignal:
			// duplicated code, just show how to handle different signals
			switch sig {
			case syscall.SIGTERM:
				log.Printf("Received SIGTERM, stopping\n")
			case syscall.SIGINT:
				log.Printf("Received SIGINT, stopping\n")
			}
			isServing = false
		case connection := <-chanConnection:
			wg.Add(1)
			go handleRequest(connection, kv)
		}
	}

	// wait till all clients disconnected
	wg.Wait()
	log.Printf("Done\n")
}

func handleRequest(connection net.Conn, kv *badger.KV) {
	defer wg.Done()
	defer connection.Close()

	remote := connection.RemoteAddr().String()
	log.Printf("Start to handle request from %s\n", remote)

	// protocol is line-oriented
	reader := bufio.NewScanner(connection)
	writer := bufio.NewWriter(connection)
	for isServing && reader.Scan() {
		op := parseRequest(reader.Text())
		if function, ok := operationMap[op.Operator]; ok {
			if function(writer, kv, op.Params...) != nil {
				break
			}
		} else {
			fmt.Fprintf(writer, "Unknown operation: %s\n", op.Operator)
		}
		writer.Flush()
	}
	if err := reader.Err(); err != nil && isServing {
		log.Printf("Failed to read request: %v\n", err)
	}
	log.Printf("End of handling request from %s\n", remote)
}

func handleGet(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// get value of a key
	if len(params) < 1 {
		fmt.Fprintf(writer, "GET need a key\n")
	} else {
		var item badger.KVItem
		if err := kv.Get([]byte(params[0]), &item); err != nil {
			fmt.Fprintf(writer, "Internal Error: %s\n", nil)
		} else if item.Value() == nil {
			fmt.Fprintf(writer, "NOT FOUND\n")
		} else {
			fmt.Fprintf(writer, "%s\n", string(item.Value()))
		}
	}
	return nil
}

func handleSet(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// set value of a key
	if len(params) < 2 {
		fmt.Fprintf(writer, "SET need key and a value\n")
	} else {
		kv.Set([]byte(params[0]), []byte(params[1]))
		fmt.Fprintf(writer, "OK\n")
	}
	return nil
}

func handleDelete(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// delete a key
	if len(params) < 1 {
		fmt.Fprintf(writer, "DELETE need a key\n")
	} else {
		kv.Delete([]byte(params[0]))
		fmt.Fprintf(writer, "OK\n")
	}
	return nil
}

func handleKeys(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// list keys that match a regular expression
	if len(params) < 1 {
		fmt.Fprintf(writer, "KEYS need a regex\n")
	} else if re, err := regexp.Compile(params[0]); err != nil {
		fmt.Fprintf(writer, "Unable to compile regex: %s\n", err.Error())
	} else {
		it := kv.NewIterator(badger.IteratorOptions{
			PrefetchSize: 1000,
			FetchValues:  false,
			Reverse:      false,
		})
		for it.Rewind(); it.Valid(); it.Next() {
			if re.Match(it.Item().Key()) {
				fmt.Fprintf(writer, "%s\n", string(it.Item().Key()))
			}
		}
	}
	return nil
}

func handleQuit(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// disconnect
	fmt.Fprintf(writer, "Bye-bye\n")
	return io.EOF
}

func handleEmpty(writer *bufio.Writer, kv *badger.KV, params ...string) error {
	// empty is a valid operator
	return nil
}

func parseRequest(request string) operation {
	op := operation{}
	re := regexp.MustCompile(`'[^']+'|"[^"]+"|\S+`)
	match := re.FindAllString(request, -1)
	if len(match) == 0 {
		return op
	}
	for index := range match {
		delimiter := ""
		if strings.HasPrefix(match[index], "'") {
			delimiter = "'"
		} else if strings.HasPrefix(match[index], `"`) {
			delimiter = `"`
		}
		match[index] = strings.TrimPrefix(match[index], delimiter)
		match[index] = strings.TrimSuffix(match[index], delimiter)
	}

	op.Operator = strings.ToUpper(match[0])
	if len(match) > 1 {
		op.Params = match[1:]
	}

	return op
}
