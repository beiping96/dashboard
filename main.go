package main

import (
	"fmt"
	"github.com/beiping96/dashboard/server"
	"os"
)

func printHelp() {
	fmt.Println("dashboard")
	fmt.Println("    ./dashboard start dashboard.xml")
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}
	switch os.Args[1] {
	case "start":
		if len(os.Args) < 3 {
			printHelp()
			return
		}
		err := server.Start(os.Args[2])
		if err != nil {
			panic(fmt.Sprintf("start failed %v", err))
		}
	default:
		printHelp()
	}
}
