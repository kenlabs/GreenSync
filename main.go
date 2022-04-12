package main

import (
	logging "github.com/ipfs/go-log/v2"
)

func main() {
	err := logging.SetLogLevel("*", "log-level")
	if err != nil {
		panic(err.Error())
	}
}
