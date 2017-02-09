# ec2backup

[![Build Status](https://img.shields.io/travis/tanis2000/ec2backup.svg)](https://travis-ci.org/tanis2000/ec2backup)

A simple Go application to automate backup of Amazon EC2 volumes.

It scans all your instances and performs backups of the attached volumes.

It supports a few options to help keeping backups under control.

# Building

Just clone this repository in your $GOPATH and run the following lines:

```
go get gopkg.in/alecthomas/kingpin.v2
go get github.com/aws/aws-sdk-go/aws
go build
```

# Usage

```
usage: ec2backup [<flags>] <region>

Flags:
      --help       Show context-sensitive help (also try --help-long and --help-man).
  -v, --verbose    Verbose mode.
  -t, --tagged     Backup only volumes tagged with the Backup=true tag
  -p, --purge      Purge old backups
  -a, --purgeauto  Purge automated backups only. Will ignore manual backups
  -d, --dryrun     Simulates creation and deletion of snapshots.
      --version    Show application version.

Args:
  <region>  AWS region.
```

# Retention policy

We currently support only one retention policy at the moment. This is something that could be expanded on. Pull requests are welcome!

The current policy is as follows:

- Keep last 30 days
- Keep first day of each month
- Keep first day of each year

Everything else is being deleted.

# Requirements

Just like every application using the AWS SDK, we have a few requirements.

## IAM User with the correct policies

This application requires an **IAM user** that can interact with Instances, Volumes and Snapshots.
Here's an example of the IAM security policy required:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1426256275000",
            "Effect": "Allow",
            "Action": [
                "ec2:CreateSnapshot",
                "ec2:CreateTags",
                "ec2:DeleteSnapshot",
                "ec2:DescribeSnapshots",
                "ec2:DescribeVolumes",
                "ec2:DescribeInstances"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
}
```

## AWS credentials

You should put the credentials of the IAM user that you're using to run this application in the correct configuration file.
Please refer to [Configuring Credentials](https://github.com/aws/aws-sdk-go#configuring-credentials)

# Manually selecting the volumes to backup

You can select manually which volumes to backup by adding a new tag called **Backup** and setting it to the value **true** on the volume that you want to backup.
You will then need to run ec2backup with the **-t** flag. This way it will only pick up those volumes that have the **Backup** tag set to **true**.

# Purging old backups

By default this application doesn't purge old backups.
You can turn it on by adding the **-p** flag.
The old backups will be purged based on the default retention policy.
Please note that all old snapshots will be checked, not just those created by this application.
If you want to delete only those created by this application, you have to add the **-a** flag. This way it will only delete those snapshots that have the **CreatedBy** tag set to **AutomatedBackup** which is set by this applicaton when the new snapshots are created.

# Simulating

The **-d** flag can be added to simulate the creation and deletion of snapshots. This is useful to check that you're actually passing the correct parameters and that everything is fine.

# License

This application is distributed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0),
see LICENSE.txt and NOTICE.txt for more information.
