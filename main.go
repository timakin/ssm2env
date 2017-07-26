package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func main() {
	injectEnviron()

	args := os.Args
	if len(args) <= 1 {
		log.Fatal("missing command")
	}

	path, err := exec.LookPath(args[1])
	if err != nil {
		log.Fatal(err)
	}
	err = syscall.Exec(path, args[1:], os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}

func trace(v ...interface{}) {
	log.Println(v...)
}

func tracef(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func injectEnviron() {
	prefix := os.Getenv("SSM2ENV_PREFIX")
	if prefix == "" {
		log.Fatal("No prefix was specified.")
		return
	}

	log.Printf("parameter prefix: %s", prefix)

	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		trace(err)
		trace("failed to create session")
		return
	}
	if *sess.Config.Region == "" {
		trace("no explict region configuration. So now retriving ec2metadata...")
		region, err := ec2metadata.New(sess).Region()
		if err != nil {
			trace(err)
			trace("could not find region configuration")
			return
		}
		sess.Config.Region = aws.String(region)
	}

	var svc *ssm.SSM
	if arn := os.Getenv("ENV_INJECTOR_ASSUME_ROLE_ARN"); arn != "" {
		creds := stscreds.NewCredentials(sess, arn)
		svc = ssm.New(sess, &aws.Config{Credentials: creds})
	} else {
		svc = ssm.New(sess)
	}

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
