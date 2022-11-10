/*
Copyright 2022 SecureWorks, Inc. All rights reserved.

Original Author: Mark Chaffe

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
version 2 as published by the Free Software Foundation.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.
*/
package main

import (
	"bytes"
	"fmt"
	"github.com/jessevdk/go-flags"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

var opts struct {
	SockAddr string `long:"socket"`
	Action   string `long:"action" choice:"renice" choice:"kill" choice:"ps" default:"ps"`
	Args     struct {
		Pid int
	} `positional-args:"yes"`
}

func main() {
	_, err := flags.Parse(&opts)

	if err != nil {
		log.Fatal(err)
	}

	if opts.SockAddr == "" {
		opts.SockAddr = filepath.Join(os.Getenv("HOME"), ".chaosbernie.sock")
	}
	socket, err := net.Dial("unix", opts.SockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer socket.Close()
	if opts.Action == "ps" {
		_, err = socket.Write([]byte("ps|"))
		if err != nil {
			log.Fatal(err)
		}
		err = socket.SetReadDeadline(time.Now().Add(time.Second * 2))
		if err != nil {
			log.Fatal(err)
		}
		var buf bytes.Buffer
		_, err = io.Copy(&buf, socket)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(buf.String())
	} else {
		log.Println(opts.Args.Pid)
		_, err = socket.Write([]byte(fmt.Sprintf("%s|%v\n", opts.Action, opts.Args.Pid)))
		if err != nil {
			log.Fatal(err)
		}
	}
}
