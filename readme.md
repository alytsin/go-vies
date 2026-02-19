# go-vies

VIES API for VAT validation written in native golang

Exposes a simple API to handle VAT validation

```go
package main

import (
	"context"
	"log"

	"github.com/alytsin/go-vies"
)

func main() {

	v, err := vies.NewValidator(nil, "")
	if err != nil {
		log.Fatalln(err)
	}

	result, err := v.Check(context.Background(), "EE100354546")
	if err != nil {
		log.Fatalln(err)
	}

	log.Println(result)
}

```