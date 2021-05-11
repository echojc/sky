package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

type config struct {
	Port int
	Name string
	Size int64
	Path string
}

func main() {
	cfg, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(cfg.Path)
	if err != nil {
		log.Fatal(err)
	}

	s := http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
	}

	doShutdown := make(chan struct{})
	waitForShutdown := make(chan struct{})

	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		v := fmt.Sprintf(`attachment; filename="%s"`, cfg.Name)
		w.Header().Set("Content-Disposition", v)
		w.Header().Set("Content-Length", strconv.FormatInt(cfg.Size, 10))
		w.WriteHeader(http.StatusOK)

		io.Copy(w, f)
		f.Close()
		close(doShutdown)
	})

	go func() {
		<-doShutdown
		if err := s.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
		close(waitForShutdown)
	}()

	addrs, _ := getIPAddrs()
	fmt.Printf("Serving '%s' on port %d...\n", cfg.Name, cfg.Port)
	fmt.Printf("IPs: %v\n", addrs)

	s.Handler = m
	if err = s.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-waitForShutdown
}

func parseArgs() (config, error) {
	var c config

	flag.IntVar(&c.Port, "p", 8888, "Listen port")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		return c, errors.New("Missing file")
	}

	// validate file exists
	path := args[0]
	s, err := os.Stat(path)
	if err != nil {
		return c, err
	}

	if s.IsDir() {
		return c, errors.New("File must be a file")
	}

	c.Path = path
	c.Name = s.Name()
	c.Size = s.Size()
	return c, nil
}

func getIPAddrs() ([]net.IP, error) {
	var addrs []net.IP

	is, err := net.Interfaces()
	if err != nil {
		return addrs, err
	}

	for _, i := range is {
		as, err := i.Addrs()
		if err != nil {
			return addrs, err
		}

		for _, a := range as {
			switch v := a.(type) {
			case *net.IPNet:
				v4 := v.IP.To4()
				if v4 != nil && !v4.IsLoopback() {
					addrs = append(addrs, v4)
				}
			}
		}
	}

	return addrs, nil
}
