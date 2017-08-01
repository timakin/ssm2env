package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var prefixOption = "SSM2ENV_PREFIX"
var verbose = flag.Bool("v", false, "verbose option")

func init() {
	flag.Parse()
}

func main() {
	prefix := os.Getenv(prefixOption)
	if prefix == "" {
		log.Fatal(fmt.Sprintf("No prefix was specified with the option: `%s`.", prefixOption))
		return
	}

	if *verbose {
		log.Printf("Parameter prefix: %s", prefix)
	}

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

	if *verbose {
		log.Println("Parsed environments from SSM...")
		for k, v := range envMap {
			log.Println(fmt.Sprintf("key: %s, value: %s", k, v))
		}
	}

	err = OutputFile(envMap)
	if err != nil {
		log.Fatal(err)
		return
	}
}
