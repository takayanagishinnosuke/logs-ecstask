package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	// AWS SDK for Go v2
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ------------------------------------------------------------------------------
// コマンドラインオプション
// ------------------------------------------------------------------------------
var (
	profile = flag.String("profile", "", "Use a specific AWS CLI profile")
	// 明示的に cluster / task を渡したい場合に備える（渡されなければ対話式）
	clusterInput = flag.String("cluster", "", "ECS Cluster name/ARN")
	taskInput    = flag.String("task", "", "ECS Task ID or ARN")
)

// ------------------------------------------------------------------------------
// Timeline 構造体: ログやイベントを時系列に追加・ソート・表示する
// ------------------------------------------------------------------------------
type Timeline struct {
	events []TimelineEvent
}

type TimelineEvent struct {
	Timestamp time.Time
	Source    string
	Message   string
}

// Add は Timeline にイベントを追加
func (tl *Timeline) Add(evt TimelineEvent) {
	tl.events = append(tl.events, evt)
}

// Print は 時系列順でソートして一括出力
func (tl *Timeline) Print() {
	// ソート
	sort.SliceStable(tl.events, func(i, j int) bool {
		return tl.events[i].Timestamp.Before(tl.events[j].Timestamp)
	})
	// 出力
	for _, e := range tl.events {
		fmt.Printf("[%s] %-12s %s\n",
			e.Timestamp.Format(time.RFC3339), e.Source, e.Message)
	}
}

// newEvent は TimelineEvent を作る簡易ヘルパー
func newEvent(ts time.Time, source, msg string) TimelineEvent {
	return TimelineEvent{
		Timestamp: ts,
		Source:    source,
		Message:   msg,
	}
}

// ------------------------------------------------------------------------------
// メインエントリーポイント
// ------------------------------------------------------------------------------
func main() {
	flag.Parse()

	// context
	ctx := context.Background()

	var cfg aws.Config
	var err error

	// AWS 設定をロード --profile が指定されていれば、その認証情報を使う
	if *profile != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(*profile))
	} else {
		// 通常はデフォルトプロファイルを使う
		cfg, err = config.LoadDefaultConfig(ctx)
	}
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// ECS / CloudWatchLogs クライアントを初期化
	ecsClient := ecs.NewFromConfig(cfg)
	logsClient := cloudwatchlogs.NewFromConfig(cfg)

	// クラスターを選択
	chosenCluster := *clusterInput
	if chosenCluster == "" {
		chosenCluster, err = chooseCluster(ctx, ecsClient)
		if err != nil {
			log.Fatalf("failed to choose cluster: %v", err)
		}
	}

	// タスクを選択
	chosenTask := *taskInput
	if chosenTask == "" {
		chosenTask, err = chooseTask(ctx, ecsClient, chosenCluster)
		if err != nil {
			log.Fatalf("failed to choose task: %v", err)
		}
	}

	// 4) ログ + サービスイベント を一括で取得・出力
	err = runTrace(ctx, ecsClient, logsClient, chosenCluster, chosenTask)
	if err != nil {
		log.Fatalf("failed to trace logs: %v", err)
	}

	fmt.Println("Done.")
}

// ------------------------------------------------------------------------------
// 対話式に ECS Cluster を選択する
// ------------------------------------------------------------------------------
func chooseCluster(ctx context.Context, ecsClient *ecs.Client) (string, error) {
	fmt.Println("Choice ECS Clusters...")

	// リストを取得
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

// ------------------------------------------------------------------------------
// 対話式に ECS Task を選択する
// ------------------------------------------------------------------------------
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

// ------------------------------------------------------------------------------
// 選択されたクラスターとタスクに対して、CloudWatch LogsとECSサービスイベントを
// 同一のタイムラインにまとめて出力する
// ------------------------------------------------------------------------------
func runTrace(ctx context.Context, ecsClient *ecs.Client, logsClient *cloudwatchlogs.Client, cluster string, taskID string) error {
	// Timeline を作成
	timeline := &Timeline{}

	// タスク情報を DescribeTasks で取得
	descOut, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{taskID},
	})
	if err != nil {
		return fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(descOut.Tasks) == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task := descOut.Tasks[0]

	taskArn := aws.ToString(task.TaskArn)
	fmt.Printf("Task ARN: %s\n", taskArn)
	fmt.Printf("Last Status: %s\n", aws.ToString(task.LastStatus))

	// サービスイベント取得 (task.Group が "service:xxx" 形式ならサービスイベントも取得)
	if groupStr := aws.ToString(task.Group); strings.HasPrefix(groupStr, "service:") {
		svcName := strings.TrimPrefix(groupStr, "service:")
		err := fetchServiceEvents(ctx, ecsClient, cluster, svcName, timeline)
		if err != nil {
			log.Printf("failed to fetch service events: %v", err)
		}
	}

	// タスク定義を DescribeTaskDefinition → CloudWatch Logs を取得して Timeline に追加
	defOut, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
	})
	if err != nil {
		return fmt.Errorf("failed to describe task definition: %w", err)
	}

	// 各コンテナ定義を確認し、awslogs ドライバの場合のみ CloudWatch Logs を取得
	for _, cdef := range defOut.TaskDefinition.ContainerDefinitions {
		if cdef.LogConfiguration == nil {
			continue
		}
		if cdef.LogConfiguration.LogDriver != ecsTypes.LogDriverAwslogs {
			continue
		}
		logGroup := cdef.LogConfiguration.Options["awslogs-group"]
		prefix := cdef.LogConfiguration.Options["awslogs-stream-prefix"]
		containerName := aws.ToString(cdef.Name)

		fullTaskID := arnToName(taskArn)
		logStream := fmt.Sprintf("%s/%s/%s", prefix, containerName, fullTaskID)

		fmt.Printf("\n=== Logs for container %s ===\n", containerName)
		err := fetchCloudWatchLogsToTimeline(ctx, logsClient, logGroup, logStream, containerName, timeline)
		if err != nil {
			log.Printf("failed to fetch logs for container=%s: %v", containerName, err)
		}
	}

	// タイムラインをまとめて出力
	fmt.Println("\n===== TIMELINE (Service Events & Logs) =====")
	timeline.Print()

	return nil
}

// ------------------------------------------------------------------------------
// サービスイベントを取得し、Timeline に追加
// ------------------------------------------------------------------------------
func fetchServiceEvents(ctx context.Context, ecsClient *ecs.Client, cluster, serviceName string, timeline *Timeline) error {
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

// ------------------------------------------------------------------------------
// CloudWatch Logs からログイベントを取得し、Timeline に追加
// ------------------------------------------------------------------------------
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

// ------------------------------------------------------------------------------
// arnToName は "arn:aws:ecs:region:accountId:task/CLUSTER_NAME/TASK_ID" などの
// スラッシュ区切りの末尾要素(TASK_ID等)を抽出
// ------------------------------------------------------------------------------
func arnToName(arn string) string {
	return arn[strings.LastIndex(arn, "/")+1:]
}
