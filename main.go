package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/charmbracelet/lipgloss"
)

// コマンドラインオプション
var (
	profile = flag.String("profile", "", "Use a specific AWS CLI profile")
	// 明示的に cluster / task を渡したい場合に備える（渡されなければ対話式）
	clusterInput = flag.String("cluster", "", "ECS Cluster name/ARN")
	taskInput    = flag.String("task", "", "ECS Task ID or ARN")
)

// スタイル定義
var (
	doneStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00"))
)

func main() {
	flag.Parse()
	ctx := context.Background()

	var cfg aws.Config
	var err error

	// AWS 設定をロード --profile が指定されていれば、その認証情報を使う
	if *profile != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(*profile))
	} else {
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

	// ログ + サービスイベント を一括で取得・出力
	err = runTrace(ctx, ecsClient, logsClient, chosenCluster, chosenTask)
	if err != nil {
		log.Fatalf("failed to trace logs: %v", err)
	}

	fmt.Println(doneStyle.Render("Done."))
}

// タスクのログとサービスイベントを取得し、Timeline に追加
func runTrace(ctx context.Context, ecsClient *ecs.Client, logsClient *cloudwatchlogs.Client, cluster string, taskID string) error {
	processor := NewTaskProcessor(ecsClient, logsClient, cluster)
	timeline := &Timeline{}

	// タスク情報取得
	descOut, err := processor.getTaskDetails(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(descOut.Tasks) == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task := descOut.Tasks[0]
	taskArn := aws.ToString(task.TaskArn)

	// サービスイベント取得
	if groupStr := aws.ToString(task.Group); strings.HasPrefix(groupStr, "service:") {
		svcName := strings.TrimPrefix(groupStr, "service:")
		if err := fetchServiceEvents(ctx, ecsClient, cluster, svcName, timeline); err != nil {
			log.Printf("failed to fetch service events: %v", err)
		}
	}

	// タスク定義取得
	defOut, err := processor.getTaskDefinition(ctx, task.TaskDefinitionArn)
	if err != nil {
		return fmt.Errorf("failed to describe task definition: %w", err)
	}

	// コンテナログ処理
	if err := processor.processContainerLogs(ctx, defOut.TaskDefinition, taskArn, timeline); err != nil {
		log.Printf("Error processing container logs: %v", err)
	}

	taskStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	fmt.Printf("%s %s\n",
		taskStyle.Render("Task ARN:"),
		taskMessageStyle.Render(taskArn))
	fmt.Printf("%s %s\n\n",
		taskStyle.Render("Last Status:"),
		taskMessageStyle.Render(aws.ToString(task.LastStatus)))

	timeline.Print()

	return nil
}
