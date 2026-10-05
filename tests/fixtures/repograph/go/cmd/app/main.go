package main

import (
	"fmt"

	"example.com/app/internal/service"
	"github.com/example/external"
)

func main() { fmt.Println(service.Run(), external.Name) }
