package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"golang.org/x/sync/errgroup"
)

var envLoaderDir = []string{"/", "etc", "profile.d"}
var envLoaderFile = "loadenv_fromssm.sh"
var envLoaderPath = append(envLoaderDir, []string{envLoaderFile}...)

type Service struct {
	SSMClient *ssm.SSM
}

func NewService() (service *Service, err error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return
	}
	if *sess.Config.Region == "" {
		log.Println("no explict region configuration. So now retriving ec2metadata...")
		region, err := ec2metadata.New(sess).Region()
		if err != nil {
			return nil, err
		}
		sess.Config.Region = aws.String(region)
	}
	var client *ssm.SSM
	if arn := os.Getenv("SSM2ENV_ASSUME_ROLE_ARN"); arn != "" {
		creds := stscreds.NewCredentials(sess, arn)
		client = ssm.New(sess, &aws.Config{Credentials: creds})
	} else {
		client = ssm.New(sess)
	}

	service = &Service{
		SSMClient: client,
	}

	return
}

func (s *Service) GetStoredKeys() ([]string, error) {
	var h = []string{}

	input := &ssm.DescribeParametersInput{}
	err := s.SSMClient.DescribeParametersPages(input,
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

func (s *Service) GetEnvMap(ctx context.Context, prefix string, keys []string) (map[string]string, error) {
	var envMap = map[string]string{}
	eg, ctx := errgroup.WithContext(ctx)

	c := make(chan map[string]string)
	defer close(c)
	for chunkedKeys := range Chunks(keys, 10) {
		chunkedKeys := chunkedKeys // https://golang.org/doc/faq#closures_and_goroutines
		eg.Go(func() error {
			m := map[string]string{}
			names := aws.StringSlice(chunkedKeys)
			result, err := s.SSMClient.GetParameters(&ssm.GetParametersInput{
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

	if err := eg.Wait(); err != nil {
		return envMap, err
	}

	for r := range c {
		envMap = Merge(envMap, r)
	}

	return envMap, nil
}

func OutputFile(envMap map[string]string) error {
	dirPath, err := filepath.Abs(filepath.Join(envLoaderDir...))
	if err != nil {
		return err
	}
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}
	}

	envFile, err := filepath.Abs(filepath.Join(envLoaderPath...))
	if err != nil {
		return err
	}

	file, err := os.Create(envFile)
	defer file.Close()
	if err != nil {
		return err
	}

	err = file.Chmod(0777)
	if err != nil {
		return err
	}

	var output string
	for key, val := range envMap {
		output = output + fmt.Sprintf("export %s=%s\n", key, val)
	}
	file.Write(([]byte)(output))

	log.Println(fmt.Sprintf("output file was placed to %s", envFile))

	return nil
}
