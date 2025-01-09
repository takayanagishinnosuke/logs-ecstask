package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// 対話式に ECS Cluster を選択する
func chooseCluster(ctx context.Context, ecsClient *ecs.Client) (string, error) {
	fmt.Println("Choice ECS Clusters...")

	var clusters []string
	var nextToken *string

	for {
		out, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return "", err
		}
		for _, arn := range out.ClusterArns {
			// arnの末尾だけ抜き出して分かりやすく表示
			cName := arnToName(arn)
			clusters = append(clusters, cName)
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	// 取得したクラスターをソートして番号付け
	sort.Strings(clusters)
	if len(clusters) == 0 {
		return "", fmt.Errorf("no ECS Clusters found")
	}

	fmt.Println("Select a cluster:")
	for i, c := range clusters {
		fmt.Printf("[%d] %s\n", i, c)
	}

	// 入力受付
	var idx int
	fmt.Print("Enter a number > ")
	_, err := fmt.Scanln(&idx)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(clusters) {
		return "", fmt.Errorf("invalid index: %d", idx)
	}

	chosen := clusters[idx]
	fmt.Printf("You chose: %s\n", chosen)
	return chosen, nil
}

// 対話式に ECS Task を選択する
func chooseTask(ctx context.Context, ecsClient *ecs.Client, cluster string) (string, error) {
	fmt.Printf("Listing Tasks in cluster: %s\n", cluster)

	// RUNNING, PENDING, STOPPED すべて取得
	statuses := []ecsTypes.DesiredStatus{
		ecsTypes.DesiredStatusRunning,
		ecsTypes.DesiredStatusPending,
		ecsTypes.DesiredStatusStopped,
	}

	var tasks []string
	for _, st := range statuses {
		tlist, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       &cluster,
			DesiredStatus: st,
		})
		if err != nil {
			return "", err
		}
		tasks = append(tasks, tlist.TaskArns...)
	}
	if len(tasks) == 0 {
		return "", fmt.Errorf("no Tasks found in cluster %s", cluster)
	}

	// タスク ARN の末尾を表示用に格納
	var taskIDs []string
	for _, arn := range tasks {
		taskIDs = append(taskIDs, arnToName(arn))
	}
	sort.Strings(taskIDs)

	fmt.Println("Select a task:")
	for i, tID := range taskIDs {
		fmt.Printf("[%d] %s\n", i, tID)
	}

	var idx int
	fmt.Print("Enter a number > ")
	_, err := fmt.Scanln(&idx)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(taskIDs) {
		return "", fmt.Errorf("invalid index: %d", idx)
	}

	chosen := taskIDs[idx]
	fmt.Printf("You chose Task: %s\n", chosen)
	return chosen, nil
}

// サービスイベントを取得し、Timeline に追加
func fetchServiceEvents(
	ctx context.Context,
	ecsClient *ecs.Client,
	cluster, serviceName string,
	timeline *Timeline,
) error {
	out, err := ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{Cluster: &cluster, Services: []string{serviceName}})
	if err != nil {
		return err
	}
	if len(out.Services) == 0 {
		return fmt.Errorf("no services found for %s", serviceName)
	}
	svc := out.Services[0]

	for _, ev := range svc.Events {
		ts := aws.ToTime(ev.CreatedAt)
		msg := aws.ToString(ev.Message)
		// Timeline に追加。ソースは "SERVICE"
		timeline.Add(newEvent(ts, "SERVICE", msg))
	}
	return nil
}

// CloudWatch Logs からログイベントを取得し、Timeline に追加
func fetchCloudWatchLogsToTimeline(
	ctx context.Context,
	logsClient *cloudwatchlogs.Client,
	group, stream, containerName string,
	timeline *Timeline,
) error {
	fmt.Printf("LogGroup: %s, LogStream: %s\n", group, stream)

	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &group,
		LogStreamName: &stream,
		StartFromHead: aws.Bool(true),
		Limit:         aws.Int32(50),
	}

	// ここでは無限ループを避けるため、最大ループ回数を区切る
	const maxIteration = 10
	iteration := 0
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
			input.StartFromHead = nil
		}
		out, err := logsClient.GetLogEvents(ctx, input)
		if err != nil {
			return err
		}

		// ログイベントを時刻順に Timeline へ追加
		for _, ev := range out.Events {
			ts := time.Unix(0, aws.ToInt64(ev.Timestamp)*int64(time.Millisecond))
			msg := aws.ToString(ev.Message)
			// ソースはコンテナ名
			timeline.Add(newEvent(ts, containerName, msg))
		}

		// トークンが同じなら終了
		if nextToken != nil && *nextToken == aws.ToString(out.NextForwardToken) {
			break
		}
		nextToken = out.NextForwardToken

		iteration++
		if iteration >= maxIteration {
			fmt.Println("Reached max iteration. Stop fetching logs.")
			break
		}
	}

	return nil
}

// コンテナ定義が awslogs ドライバを使っているか判定
func isAwslogsDriver(logConfig *ecsTypes.LogConfiguration) bool {
	return logConfig != nil && logConfig.LogDriver == ecsTypes.LogDriverAwslogs
}

// arnからスラッシュ区切りの末尾要素(TASK_ID等)を抽出
func arnToName(arn string) string {
	return arn[strings.LastIndex(arn, "/")+1:]
}
