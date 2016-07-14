package main

import (
	"fmt"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var (
	verbose            = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	region             = kingpin.Arg("region", "AWS region.").Required().String()
	taggedOnly         = kingpin.Flag("tagged", "Backup only volumes tagged with the Backup=true tag").Short('t').Bool()
	purge              = kingpin.Flag("purge", "Purge old backups").Short('p').Bool()
	purgeAutomatedOnly = kingpin.Flag("purgeauto", "Purge automated backups only. Will ignore manual backups").Short('a').Bool()
	dry                = kingpin.Flag("dryrun", "Simulates creation and deletion of snapshots.").Short('d').Bool()
)

func main() {
	kingpin.Version("0.1.0")
	kingpin.Parse()
	fmt.Printf("Selected region: %s\n", *region)
	fmt.Println("Current date and time: ", time.Now())
	if *taggedOnly {
		fmt.Println("Only volumes tagged with Backup=true will be backed up")
	}
	if *purge {
		fmt.Println("Purging old backups with default policy (last 30 days and 1st day of each month and 1st day of each year)")
		if *purgeAutomatedOnly {
			fmt.Println("Purging automated backups only")
		}
	} else {
		fmt.Println("Won't purge")
	}
	if *dry {
		fmt.Println("Dry run. We will simulate creation and deletion commands")
	}

	svc := ec2.New(session.New(), &aws.Config{Region: aws.String(*region)})

	resp, err := svc.DescribeInstances(nil)
	if err != nil {
		panic(err)
	}

	snapsDeletedCounter := 0
	volumesSnapshottedCounter := 0

	fmt.Println("> Number of reservation sets: ", len(resp.Reservations))
	for idx, res := range resp.Reservations {
		fmt.Println("  > Number of instances: ", len(res.Instances))
		for _, inst := range resp.Reservations[idx].Instances {
			fmt.Println("    - Instance ID: ", *inst.InstanceId, " - State: ", *inst.State.Name)
			for _, tag := range inst.Tags {
				fmt.Println("      - Tag key: ", *tag.Key, " - Value: ", *tag.Value)
			}
			for _, block := range inst.BlockDeviceMappings {
				fmt.Println("      - Device: ", *block.DeviceName, " - Volume: ", *block.Ebs.VolumeId)
				dvi := &ec2.DescribeVolumesInput{VolumeIds: []*string{block.Ebs.VolumeId}}
				volresp, err := svc.DescribeVolumes(dvi)
				if err != nil {
					panic(err)
				}
				for _, vres := range volresp.Volumes {
					if *purge {
						snapshots, err := ListSnapshots(svc, vres.VolumeId, *purgeAutomatedOnly)
						if err != nil {
							panic(err)
						}
						if *purgeAutomatedOnly {
							snapshots, err = AutomatedSnapshotsOnly(snapshots)
						}
						for _, snap := range snapshots {
							fmt.Println("Checking snapshot ", *snap.SnapshotId, " with date ", *snap.StartTime)
							keep, err := ShouldKeep(snap)
							if err != nil {
								panic(err)
							}
							if !keep {
								_, err := DeleteSnapshot(svc, snap.SnapshotId)
								if err != nil {
									panic(err)
								}
								snapsDeletedCounter++
							}
						}
					}
					name := ""
					backupTag := false
					for _, vtag := range vres.Tags {
						fmt.Println("        - Tag key: ", *vtag.Key, " - Value: ", *vtag.Value)
						if *vtag.Key == "Name" {
							name = *vtag.Value
						}
						if *vtag.Key == "Backup" {
							backupString := *vtag.Value
							if backupString == "true" {
								backupTag = true
							}
						}
					}
					if !*taggedOnly || backupTag {
						createRes, err := CreateSnapshot(svc, vres.VolumeId, &name)
						if err != nil {
							panic(err)
						}
						if createRes {
							volumesSnapshottedCounter++
						}
					}
				}
			}
		}
	}

	fmt.Println(snapsDeletedCounter, " snapshots deleted.")
	fmt.Println(volumesSnapshottedCounter, " volumes snapshots created.")
}

func DeleteSnapshot(svc *ec2.EC2, snapID *string) (bool, error) {
	fmt.Println("Deleting snapshot ", *snapID)

	if *dry {
		fmt.Println("!!!SIMULATION ONLY!!!")
		return true, nil
	}

	in := ec2.DeleteSnapshotInput{SnapshotId: snapID}
	_, err := svc.DeleteSnapshot(&in)
	if err != nil {
		return false, err
	}

	return true, nil
}

func ListSnapshots(svc *ec2.EC2, volumeID *string, automatedOnly bool) ([]*ec2.Snapshot, error) {
	volFilter := ec2.Filter{Name: aws.String("volume-id"), Values: []*string{volumeID}}
	filter := []*ec2.Filter{&volFilter}
	in := ec2.DescribeSnapshotsInput{Filters: filter}
	res, err := svc.DescribeSnapshots(&in)
	if err != nil {
		return nil, err
	}

	return res.Snapshots, nil
}

func AutomatedSnapshotsOnly(snapshots []*ec2.Snapshot) ([]*ec2.Snapshot, error) {
	res := []*ec2.Snapshot{}
	for _, snapshot := range snapshots {
		for _, tag := range snapshot.Tags {
			if *tag.Key == "CreatedBy" && *tag.Value == "AutomatedBackup" {
				res = append(res, snapshot)
			}
		}
	}
	return res, nil
}

func ShouldKeep(snapshot *ec2.Snapshot) (bool, error) {
	year, month, day := snapshot.StartTime.Date()

	// 1st day of the year
	if month == 1 && day == 1 {
		return true, nil
	}

	// 1st day of month
	if day == 1 {
		return true, nil
	}

	// All the days of the current month (of the current year, of course)
	n := time.Now()
	thisYear, thisMonth, _ := n.Date()
	if year == thisYear && month == thisMonth {
		return true, nil
	}

	return false, nil
}

func CreateSnapshot(svc *ec2.EC2, volumeID *string, name *string) (bool, error) {
	fmt.Println("Created snapshot of volume ", *volumeID)

	if *dry {
		fmt.Println("!!!SIMULATION ONLY!!!")
		return true, nil
	}

	desc := volumeID
	dryRun := false

	snapshot := &ec2.CreateSnapshotInput{Description: desc, VolumeId: volumeID, DryRun: &dryRun}
	res, err := svc.CreateSnapshot(snapshot)
	if err != nil {
		return false, err
	}

	fmt.Println("Created snapshot: ", *res.SnapshotId, " - Date of creation: ", *res.StartTime)
	tag := ec2.Tag{Key: aws.String("Name"), Value: name}
	createdTag := ec2.Tag{Key: aws.String("CreatedBy"), Value: aws.String("AutomatedBackup")}
	createTag := &ec2.CreateTagsInput{Resources: []*string{res.SnapshotId}, Tags: []*ec2.Tag{&tag, &createdTag}}
	_, err = svc.CreateTags(createTag)
	if err != nil {
		return false, err
	}

	return true, nil
}
