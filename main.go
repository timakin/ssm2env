package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/k0kubun/pp"
	"golang.org/x/sync/errgroup"
)

var prefixOption = "SSM2ENV_PREFIX"

func main() {
	injectEnv()
}

func injectEnv() {
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

func getSSMService() (svc *ssm.SSM, err error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	if *sess.Config.Region == "" {
		log.Println("no explict region configuration. So now retriving ec2metadata...")
		region, err := ec2metadata.New(sess).Region()
		if err != nil {
			return nil, err
		}
		sess.Config.Region = aws.String(region)
	}
	if arn := os.Getenv("ENV_INJECTOR_ASSUME_ROLE_ARN"); arn != "" {
		creds := stscreds.NewCredentials(sess, arn)
		svc = ssm.New(sess, &aws.Config{Credentials: creds})
	} else {
		svc = ssm.New(sess)
	}
	return
}

func getStoredKeys(svc *ssm.SSM) ([]string, error) {
	var h = []string{}

	input := &ssm.DescribeParametersInput{}
	err := svc.DescribeParametersPages(input,
		func(page *ssm.DescribeParametersOutput, lastPage bool) bool {
			for i := range page.Parameters {
				param := page.Parameters[i]
				h = append(h, *param.Name)
			}
			return !lastPage
		})
	if err != nil {
		return []string{}, err
	}

	return h, nil
}

func getEnvMap(ctx context.Context, svc *ssm.SSM, prefix string, keys []string) (map[string]string, error) {
	var envMap = map[string]string{}
	eg, ctx := errgroup.WithContext(context.Background())

	c := make(chan map[string]string)
	for chunkedKeys := range Chunks(keys, 10) {
		chunkedKeys := chunkedKeys // https://golang.org/doc/faq#closures_and_goroutines
		eg.Go(func() error {
			m := map[string]string{}
			names := aws.StringSlice(chunkedKeys)
			result, err := svc.GetParameters(&ssm.GetParametersInput{
				Names:          names,
				WithDecryption: aws.Bool(true),
			})
			if err != nil {
				return err
			}

			oldKey := fmt.Sprintf("%s.", prefix)
			newKey := ""
			replacer := strings.NewReplacer(oldKey, newKey)
			for i := range result.Parameters {
				param := result.Parameters[i]
				key := replacer.Replace(*param.Name)
				val := *param.Value
				m[key] = val
			}
			select {
			case c <- m:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}
	go func() {
		eg.Wait()
		close(c)
	}()

	for r := range c {
		envMap = Merge(envMap, r)
	}

	return envMap, nil
}

func Filter(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func Chunks(l []string, n int) chan []string {
	ch := make(chan []string)

	go func() {
		for i := 0; i < len(l); i = i + n {
			fromIdx := i
			toIdx := i + n
			if toIdx > len(l) {
				toIdx = len(l)
			}
			ch <- l[fromIdx:toIdx]
		}
		close(ch)
	}()
	return ch
}

func Merge(m1, m2 map[string]string) map[string]string {
	ans := map[string]string{}

	for k, v := range m1 {
		ans[k] = v
	}
	for k, v := range m2 {
		ans[k] = v
	}
	return (ans)
}
