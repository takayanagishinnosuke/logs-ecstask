package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type TaskProcessor struct {
	ecsClient  *ecs.Client
	logsClient *cloudwatchlogs.Client
	cluster    string
}

func NewTaskProcessor(
	ecsClient *ecs.Client,
	logsClient *cloudwatchlogs.Client,
	cluster string) *TaskProcessor {
	return &TaskProcessor{
		ecsClient:  ecsClient,
		logsClient: logsClient,
		cluster:    cluster,
	}
}

// ECS タスクの詳細を取得する
func (p *TaskProcessor) getTaskDetails(
	ctx context.Context, taskID string) (*ecs.DescribeTasksOutput, error) {
	return p.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &p.cluster,
		Tasks:   []string{taskID},
	})
}

// ECS タスク定義を取得する
func (p *TaskProcessor) getTaskDefinition(
	ctx context.Context, taskDefArn *string) (*ecs.DescribeTaskDefinitionOutput, error) {
	return p.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefArn,
	})
}

// タスク定義からロググループを取得し、CloudWatch Logs からログを取得
func (p *TaskProcessor) processContainerLogs(
	ctx context.Context,
	def *ecsTypes.TaskDefinition,
	taskArn string,
	timeline *Timeline,
) error {
	for _, cdef := range def.ContainerDefinitions {
		if !isAwslogsDriver(cdef.LogConfiguration) {
			continue
		}

		logGroup := cdef.LogConfiguration.Options["awslogs-group"]
		prefix := cdef.LogConfiguration.Options["awslogs-stream-prefix"]
		containerName := aws.ToString(cdef.Name)

		fullTaskID := arnToName(taskArn)
		logStream := fmt.Sprintf("%s/%s/%s", prefix, containerName, fullTaskID)

		if err := fetchCloudWatchLogsToTimeline(ctx, p.logsClient, logGroup, logStream, containerName, timeline); err != nil {
			return fmt.Errorf("failed to fetch logs for container=%s: %w", containerName, err)
		}
	}
	return nil
}
