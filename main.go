package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/k0kubun/pp"
)

var prefixOption = "SSM2ENV_PREFIX"

func main() {
	_main()
}

func _main() {
	prefix := os.Getenv(prefixOption)
	if prefix == "" {
		log.Fatal(fmt.Sprintf("No prefix was specified with the option: `%f`.", prefixOption))
		return
	}

	log.Printf("parameter prefix: %s", prefix)

	svc, err := getSSMService()
	if err != nil {
		log.Fatal(err)
		return
	}

	allKeys, err := getStoredKeys(svc)
	if err != nil {
		log.Fatal(err)
		return
	}

	keys := Filter(allKeys, func(v string) bool {
		return strings.HasPrefix(v, prefix)
	})

	ctx := context.Background()
	envMap, err := getEnvMap(ctx, svc, prefix, keys)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Print(pp.Sprint(envMap))

	// for _, key := range result.InvalidParameters {
	// 	tracef("invalid parameter: %s", *key)
	// }
	// for _, param := range result.Parameters {
	// 	key := strings.TrimPrefix(*param.Name, prefix)
	// 	os.Setenv(key, *param.Value)
	// 	tracef("env injected: %s", key)
	// }
}
