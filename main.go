package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

var prefixOption = "SSM2ENV_PREFIX"

func main() {
	prefix := os.Getenv(prefixOption)
	if prefix == "" {
		log.Fatal(fmt.Sprintf("No prefix was specified with the option: `%s`.", prefixOption))
		return
	}

	log.Printf("parameter prefix: %s", prefix)

	svc, err := NewService()
	if err != nil {
		log.Fatal(err)
		return
	}

	allKeys, err := svc.GetStoredKeys()
	if err != nil {
		log.Fatal(err)
		return
	}

	keys := Filter(allKeys, func(v string) bool {
		return strings.HasPrefix(v, prefix)
	})

	ctx := context.Background()
	envMap, err := svc.GetEnvMap(ctx, prefix, keys)
	if err != nil {
		log.Fatal(err)
		return
	}

	err = OutputFile(envMap)
	if err != nil {
		log.Fatal(err)
		return
	}
}
