# ssm2env
Environments injection tool with parameters fetched from AWS (Amazon System Manager EC2 Parameter Store).

# Install

```
$ go get -u github.com/timakin/ssm2env
```

# Prepare

At first, set your own parameters to SSM, EC2 parameter store.

Notice: Please set the prefix for parameters with dots. 

ex) `testapi.prod.DB_PASSWORD`

```
$ aws ssm get-parameters --with-decryption --names testapi.prod.DB_USER testapi.prod.DB_PASSWORD
{
    "InvalidParameters": [],
    "Parameters": [
        {
            "Type": "String",
            "Name": "testapi.prod.DB_USER",
            "Value": "root"
        },
        {
            "Type": "SecureString",
            "Name": "testapi.prod.DB_PASSWORD",
            "Value": "password"
        }
    ]
}
```

# Usage

Inside your server with aws credentials, just type `SSM2ENV_PREFIX=testapi.prod ssm2env` .

This command will generate the script to export the environments.

After reproduction a process, init script will be called and it'll export above variables.

If you'd set envs immediately, just call the raw script file. `source /etc/profile.d/loadenv_fromssm.sh`
