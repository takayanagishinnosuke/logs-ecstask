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
	"github.com/charmbracelet/lipgloss"
)

// タスク詳細情報
type TaskDisplay struct {
	ID         string
	Definition string
	FullArn    string
}

// スタイル定義
var (
	waitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080"))

	choiceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff00"))

	nomberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00bfff"))

	idStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080"))

	aggregateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffff00"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0000"))
)

// 対話式に ECS Cluster を選択する
func chooseCluster(ctx context.Context, ecsClient *ecs.Client) (string, error) {
	fmt.Println(waitStyle.Render("Listing ECS Clusters..."))

	clusters, err := fetchClusters(ctx, ecsClient)
	if err != nil {
		return "", err
	}
	displayClusters(clusters)

	// 入力受付
	var idx int
	fmt.Print(choiceStyle.Render("Enter a number ➡ "))
	_, err = fmt.Scanln(&idx)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(clusters) {
		return "", fmt.Errorf(errorStyle.Render("invalid index"))
	}

	chosen := clusters[idx]
	styledText := aggregateStyle.Render(fmt.Sprintf("You chose: %s\n", chosen))
	fmt.Println(styledText)
	return chosen, nil
}

// AWSからクラスター一覧を取得
func fetchClusters(ctx context.Context, ecsClient *ecs.Client) ([]string, error) {
	var clusters []string
	var nextToken *string

	for {
		out, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, arn := range out.ClusterArns {
			cName := arnToName(arn)
			clusters = append(clusters, cName)
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf(errorStyle.Render("no clusters found"))
	}
	sort.Strings(clusters)
	return clusters, nil
}

// クラスター一覧を表示
func displayClusters(clusters []string) {
	fmt.Println(choiceStyle.Render("Select a cluster 👇"))
	for i, c := range clusters {
		numberStr := fmt.Sprintf("[%d]", i)
		line := fmt.Sprintf("%s %s",
			nomberStyle.Render(numberStr),
			idStyle.Render(c),
		)
		fmt.Println(line)
	}
}

// 対話式に ECS タスクを選択する
func chooseTask(ctx context.Context, ecsClient *ecs.Client, cluster string) (string, error) {
	fmt.Println(waitStyle.Render("Listing Task..."))

	taskArns, err := listTaskArns(ctx, ecsClient, cluster)
	if err != nil {
		return "", err
	}

	tasks, err := getTaskDetails(ctx, ecsClient, cluster, taskArns)
	if err != nil {
		return "", err
	}

	displayTasks(tasks)

	// 入力受付
	var idx int
	fmt.Print(choiceStyle.Render("Enter a number ➡ "))
	_, err = fmt.Scanln(&idx)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(tasks) {
		return "", fmt.Errorf(errorStyle.Render("invalid index"))
	}

	chosen := tasks[idx].FullArn
	fmt.Println(aggregateStyle.Render("You chose Task:", tasks[idx].ID))
	return chosen, nil
}

// ECSタスクの一覧を取得
func listTaskArns(ctx context.Context, ecsClient *ecs.Client, cluster string) ([]string, error) {
	statuses := []ecsTypes.DesiredStatus{
		ecsTypes.DesiredStatusRunning,
		ecsTypes.DesiredStatusPending,
		ecsTypes.DesiredStatusStopped,
	}

	var taskArns []string
	for _, st := range statuses {
		tlist, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       &cluster,
			DesiredStatus: st,
		})
		if err != nil {
			return nil, err
		}
		taskArns = append(taskArns, tlist.TaskArns...)
	}
	if len(taskArns) == 0 {
		return nil, fmt.Errorf(errorStyle.Render("no Tasks found in cluster", cluster))
	}
	return taskArns, nil
}

// タスク定義情報を取得
func getTaskDetails(ctx context.Context, ecsClient *ecs.Client, cluster string, taskArns []string) ([]TaskDisplay, error) {
	descOutput, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   taskArns,
	})
	if err != nil {
		return nil, err
	}

	var tasks []TaskDisplay
	for _, task := range descOutput.Tasks {
		id := arnToName(aws.ToString(task.TaskArn))
		defName := ""
		if task.TaskDefinitionArn != nil {
			defArn := aws.ToString(task.TaskDefinitionArn)
			defName = strings.Split(defArn, "task-definition/")[1]
		}
		tasks = append(tasks, TaskDisplay{
			ID:         id,
			Definition: defName,
			FullArn:    aws.ToString(task.TaskArn),
		})
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

// ECSタスク一覧を表示する
func displayTasks(tasks []TaskDisplay) {
	fmt.Println(choiceStyle.Render("Select a Task 👇"))
	for i, t := range tasks {
		numberStr := fmt.Sprintf("[%d]", i)
		line := fmt.Sprintf("%s %s: %s",
			nomberStyle.Render(numberStr),
			idStyle.Render(t.ID),
			idStyle.Render(t.Definition),
		)
		fmt.Println(line)
	}
}

// サービスイベントを取得し、Timeline に追加
func fetchServiceEvents(ctx context.Context, ecsClient *ecs.Client, cluster, serviceName string, timeline *Timeline) error {
	out, err := ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{Cluster: &cluster, Services: []string{serviceName}})
	if err != nil {
		return err
	}
	if len(out.Services) == 0 {
		errorText := errorStyle.Render("no services found for", serviceName)
		return fmt.Errorf(errorText)
	}
	svc := out.Services[0]

	// 最新10件のイベントのみを処理
	// const recentEventsLimit = 10
	// events := svc.Events
	// if len(events) > recentEventsLimit {
	// 	events = events[:recentEventsLimit]
	// }

	for _, ev := range svc.Events {
		ts := aws.ToTime(ev.CreatedAt)
		msg := aws.ToString(ev.Message)
		// Timeline に追加。ソースは "SERVICE"
		timeline.Add(newEvent(ts, "SERVICE", msg))
	}
	return nil
}

// CloudWatch Logs からログイベントを取得し、Timeline に追加
func fetchCloudWatchLogsToTimeline(ctx context.Context, logsClient *cloudwatchlogs.Client, group, stream, containerName string, timeline *Timeline) error {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &group,
		LogStreamName: &stream,
		StartFromHead: aws.Bool(false),
		Limit:         aws.Int32(40),
	}

	// API呼び出しの最大回数
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
			fmt.Println(aggregateStyle.Render("Reached max iteration"))
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
