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
	"golang.org/x/sync/errgroup"
)

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
