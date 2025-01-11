package main

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// -----------------------------------------------------------------------------
// このテストでは、ecs.Client、cloudwatchlogs.Client、およびクラスター名が正しく設定されているかを確認します。
// ecsClient、logsClient、および cluster が期待通りに設定されていない場合、エラーメッセージを出力します。
// -----------------------------------------------------------------------------
func TestNewTaskProcessor(t *testing.T) {
	ecsClient := &ecs.Client{}
	logsClient := &cloudwatchlogs.Client{}
	cluster := "test-cluster"

	processor := NewTaskProcessor(ecsClient, logsClient, cluster)

	if processor.ecsClient != ecsClient {
		t.Error("ecsClient was not properly set")
	}
	if processor.logsClient != logsClient {
		t.Error("logsClient was not properly set")
	}
	if processor.cluster != cluster {
		t.Errorf("cluster = %s, want %s", processor.cluster, cluster)
	}
}

// -----------------------------------------------------------------------------
// isAwslogsDriver 関数の動作をテストします。
// テストケース:
// 1. "nil config": LogConfiguration が nil の場合
// 2. "awslogs driver": LogDriver が ecsTypes.LogDriverAwslogs の場合 (正常系)
// 3. "other driver": LogDriver が "json-file" の場合
// -----------------------------------------------------------------------------
func TestIsAwslogsDriver(t *testing.T) {
	testCases := []struct {
		name     string
		config   *ecsTypes.LogConfiguration
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: false,
		},
		{
			name: "awslogs driver",
			config: &ecsTypes.LogConfiguration{
				LogDriver: ecsTypes.LogDriverAwslogs,
			},
			expected: true,
		},
		{
			name: "other driver",
			config: &ecsTypes.LogConfiguration{
				LogDriver: "json-file",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isAwslogsDriver(tc.config)
			if got != tc.expected {
				t.Errorf("isAwslogsDriver() = %v, want %v", got, tc.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Timeline 構造体にイベントを追加し、それらが正しくソートされているかを確認するテストです。
// テスト内容:
// 1. テストデータとして2つのイベントを作成します。
// 2. それらのイベントを Timeline に追加します。
// 3. Timeline のイベント数が期待通りであることを確認します。
// 4. Timeline の Print メソッドを呼び出し、イベントがタイムスタンプ順にソートされていることを確認します。
// -----------------------------------------------------------------------------
func TestTimelineAddAndPrint(t *testing.T) {
	tl := &Timeline{}

	// テストデータ作成
	events := []TimelineEvent{
		{
			Timestamp: time.Date(2023, 1, 2, 15, 04, 05, 0, time.UTC),
			Source:    "TEST1",
			Message:   "Message 1",
		},
		{
			Timestamp: time.Date(2023, 1, 1, 15, 04, 05, 0, time.UTC),
			Source:    "TEST2",
			Message:   "Message 2",
		},
	}

	// イベント追加
	for _, e := range events {
		tl.Add(e)
	}

	// 要素数の確認
	if len(tl.events) != 2 {
		t.Errorf("expected 2 events, got %d", len(tl.events))
	}

	// ソート後、最初のイベントが古い方になることを確認
	tl.Print() // ソートを実行
	if tl.events[0].Timestamp.After(tl.events[1].Timestamp) {
		t.Error("events are not properly sorted")
	}
}

// -----------------------------------------------------------------------------
// 正しくEvent構造体を初期化することをテストします。作成されたEventのTimestamp、Source、
// およびMessageフィールドが期待される値と一致することを確認します。
// -----------------------------------------------------------------------------
func TestNewEvent(t *testing.T) {
	ts := time.Now()
	source := "TEST"
	msg := "test message"

	event := newEvent(ts, source, msg)

	if event.Timestamp != ts {
		t.Errorf("expected timestamp %v, got %v", ts, event.Timestamp)
	}
	if event.Source != source {
		t.Errorf("expected source %s, got %s", source, event.Source)
	}
	if event.Message != msg {
		t.Errorf("expected message %s, got %s", msg, event.Message)
	}
}

// -----------------------------------------------------------------------------
// このテストは、以下のケースを検証します:
// 1. タスク ARN からタスク ID を抽出するケース
// 2. クラスター ARN からクラスター名を抽出するケース
// 3. シンプルなパスからリソース名を抽出するケース
// 各テストケースでは、期待される出力と実際の出力を比較し、一致しない場合はエラーメッセージを表示します。
// -----------------------------------------------------------------------------
func TestArnToName(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "task ARN",
			arn:      "arn:aws:ecs:region:account:task/cluster/task-id",
			expected: "task-id",
		},
		{
			name:     "cluster ARN",
			arn:      "arn:aws:ecs:region:account:cluster/cluster-name",
			expected: "cluster-name",
		},
		{
			name:     "simple path",
			arn:      "path/to/resource",
			expected: "resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := arnToName(tt.arn)
			if got != tt.expected {
				t.Errorf("arnToName(%s) = %s; want %s", tt.arn, got, tt.expected)
			}
		})
	}
}
