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
	"encoding/json"
	"fmt"
	"github.com/emicklei/dot"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ResourceStatus int

const (
	Dead ResourceStatus = iota
	Shot
	Alive
)

type VirtualMachine struct {
	Id            string    `json:"id"`
	Location      string    `json:"location"`
	Name          string    `json:"name"`
	TimeCreated   time.Time `json:"timeCreated"`
	Type          string    `json:"type"`
	VmId          string    `json:"vmId"`
	ResourceGroup string    `json:"resourceGroup"`
}

type Proc struct {
	User   string // resource group
	Pid    int    // unique id
	Name   string // name
	Daemon int    // 1 for daemon, 0 for process
	Type   string // azure type
	Status ResourceStatus
}

func updateScore() {
	g := dot.NewGraph(dot.Directed)
	g.ID("resources")
	// hack to set graph properties
	graph := g.Node("Azure Resources")
	graph.Attr("layout", "neato")
	graph.Attr("overlap", "prism")

	coreNode := g.Node("Azure").Attr("style", "filled").Attr("fillcolor", "lightblue")

	for _, proc := range azr.procs {
		var nodeColour string
		switch proc.Status {
		case Alive:
			nodeColour = "green3"
		case Shot:
			nodeColour = "orange"
		case Dead:
			nodeColour = "orangered"
		}

		rgNode := g.Node(proc.User).Attr("style", "filled")
		if strings.EqualFold("Microsoft.Compute/virtualMachines", proc.Type) {
			rgNode.Attr("fillcolor", "lightblue")
			vmNode := g.Node(proc.Name).Attr("style", "filled").Attr("fillcolor", nodeColour)
			g.Edge(rgNode, vmNode)
		} else {
			rgNode.Attr("fillcolor", nodeColour)
		}
		if len(coreNode.EdgesTo(rgNode)) == 0 {
			g.Edge(coreNode, rgNode)
		}
	}

	file, err := os.Create("resources.gv")
	if err != nil {
		log.Fatalln(err)
	}
	// hack to set graph properties
	graphString := strings.Replace(g.String(), "n1[", "graph[", 1)
	_, err = file.WriteString(graphString)
	if err != nil {
		log.Fatalln(err)
	}
}

func (ar *AzResources) getProc(pid int) (*Proc, error) {
	ar.mutex.RLock()
	defer ar.mutex.RUnlock()
	// probably a better way to do this
	idx := -1
	for i := range azr.procs {
		if azr.procs[i].Pid == pid {
			idx = i
		}
	}
	if idx == -1 {
		return nil, errors.New("pid not in process list")
	}
	return &ar.procs[idx], nil
}

func (ar *AzResources) updateStatus(proc *Proc, status ResourceStatus) {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()
	proc.Status = status
	log.Debugf("%s %d %s %d [%d]\n", strings.ToLower(proc.User), proc.Pid, proc.Name, proc.Daemon, proc.Status)
	updateScore()
	return
}

type AzResources struct {
	procs []Proc
	mutex sync.RWMutex
}

func (ar *AzResources) UnmarshalJSON(buf []byte) error {
	var vms []VirtualMachine
	if err := json.Unmarshal(buf, &vms); err != nil {
		return errors.Wrap(err, "parsing into VirtualMachine failed")
	}
	var tmp []Proc
	for i, vm := range vms {
		var r Proc
		r.User = vm.ResourceGroup
		r.Pid = i + 100
		r.Name = vm.Name
		r.Type = vm.Type
		if opts.Action == "delete" {
			r.Daemon = 0
		} else {
			r.Daemon = 1
		}
		r.Status = Alive
		tmp = append(tmp, r)
	}
	ar.procs = tmp
	log.Infof("loaded %v resources\n", len(tmp))
	return nil
}

func RunAzureCommand(arg string) {
	pid, err := strconv.Atoi(arg)
	if err != nil {
		log.Errorln("invalid integer")
		return
	}
	proc, err := azr.getProc(pid)
	if err != nil {
		log.Errorln(err)
		return
	}
	log.Debugf("%#v", proc)
	azr.updateStatus(proc, Shot)
	var cmd string
	if strings.EqualFold("Microsoft.Compute/virtualMachines", proc.Type) {
		cmd = fmt.Sprintf("az vm %s --resource-group %s --name %s", opts.Action, proc.User, proc.Name)
		if opts.Action == "delete" {
			cmd += " -y --force-deletion true"
		}
	} else if strings.EqualFold("Microsoft.Resources/Subscriptions/ResourceGroups", proc.Type) && opts.Action == "delete" {
		cmd = fmt.Sprintf("az group delete -y --resource-group %s", proc.Name)
	} else {
		log.Warnln("no valid azure action, skipping")
		return
	}
	log.Debugf("Running: %s\n", cmd)
	if opts.DryRun {
		log.Warnln("dry run, skipping execution")
		time.Sleep(10 * time.Second)
	} else {
		execCmd := exec.Command("bash", "-c", cmd)
		execCmd.Env = os.Environ()
		if err := execCmd.Run(); err != nil {
			log.Errorln(err)
		}
	}
	azr.updateStatus(proc, Dead)
	return
}

var cmdre = regexp.MustCompile(`^(?P<cmd>\w+)\|(?P<value>\S*)?`)

func handler(conn net.Conn) {
	log.Debugf("Client connected [%s]", conn.RemoteAddr().Network())
	err := conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	if err != nil {
		log.Fatal(err)
	}
	buffer := make([]byte, 1024)
	nread, err := conn.Read(buffer)
	defer conn.Close()
	if err != nil {
		log.Errorf("failed to read from socket: %s\n", err)
		return
	}
	log.Debugf("bytes read: %d, content: %s\n", nread, buffer)
	match := cmdre.FindStringSubmatch(string(buffer))
	if len(match) == 0 {
		log.Errorln("invalid command: unable to match regexp")
		return
	}

	result := make(map[string]string)
	for i, name := range cmdre.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	if len(result) == 0 {
		log.Errorln("invalid command")
		return
	}

	switch result["cmd"] {
	case "ps":
		log.Infoln("PS")
		for _, proc := range azr.procs {
			if proc.Status != Alive {
				continue
			}
			// format of data is as follows
			// <user> <pid> <processname> <is_daemon=[1|0]>
			_, err = conn.Write([]byte(fmt.Sprintf("%s %d %s %d\n", strings.ToLower(proc.User), proc.Pid, proc.Name, proc.Daemon)))
			if err != nil {
				log.Errorln(err)
			}
		}
	case "renice":
		log.Infof("RENICE: %v\n", result["value"])
		go RunAzureCommand(result["value"])
	case "kill":
		log.Infof("KILL: %v\n", result["value"])
		go RunAzureCommand(result["value"])
	default:
		log.Warnln("action not found")
	}
}

var opts struct {
	File     string `short:"f" long:"file" description:"JSON file with resources" value-name:"FILE" required:"true"`
	Action   string `long:"action" choice:"deallocate" choice:"delete" default:"deallocate"`
	SockAddr string `long:"socket" description:"unix domain socket to create, defaults to ~/.chaosbernie.sock"`
	Debug    bool   `short:"d" long:"debug"`
	DryRun   bool   `long:"dry-run" description:"don't perform any actions on Azure resources"`
}

var azr AzResources

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.InfoLevel)

	_, err := flags.Parse(&opts)
	if err != nil {
		log.Fatal(err)
	}

	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}

	syscall.Umask(0077)
	if opts.SockAddr == "" {
		opts.SockAddr = filepath.Join(os.Getenv("HOME"), ".chaosbernie.sock")
	}

	if err = os.RemoveAll(opts.SockAddr); err != nil {
		log.Fatal(err)
	}

	jsonb, err := os.ReadFile(opts.File)
	if err != nil {
		log.Fatal(err)
	}

	if err = json.Unmarshal(jsonb, &azr); err != nil {
		log.Fatalln("error parsing JSON", err)
	}

	l, err := net.Listen("unix", opts.SockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer l.Close()

	updateScore()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		go handler(conn)
	}
}
