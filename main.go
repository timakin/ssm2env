package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/k0kubun/pp"
)

func main() {
	injectEnv()
}

func trace(v ...interface{}) {
	log.Println(v...)
}

func tracef(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func injectEnv() {
	prefix := os.Getenv("SSM2ENV_PREFIX")
	if prefix == "" {
		log.Fatal("No prefix was specified.")
		return
	}

	log.Printf("parameter prefix: %s", prefix)

	svc, err := getSSMService()
	if err != nil {
		log.Fatal(err)
		return
	}

	keys, err := getStoredKeys(svc)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Print(pp.Sprint(keys))

	// result, err := svc.GetParameters(&ssm.GetParametersInput{
	// 	Names:          names,
	// 	WithDecryption: aws.Bool(true),
	// })
	// if err != nil {
	// 	trace(err)
	// 	return
	// }

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
