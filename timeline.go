package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5f5f5")).
			Width(25).MarginRight(2)

	sourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00bfff")).
			Width(25)

	taskMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f5f5f5"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5f5f5")).
			Width(45).
			MarginBottom(2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00bfff")).
			Bold(true).
			Width(25).MarginRight(2).MarginBottom(3)
)

type Timeline struct {
	events []TimelineEvent
}

type TimelineEvent struct {
	Timestamp time.Time
	Source    string
	Message   string
}

// Timeline にイベントを追加
func (tl *Timeline) Add(evt TimelineEvent) {
	tl.events = append(tl.events, evt)
}

// 時系列順でソートして一括出力
func (tl *Timeline) Print() {
	// ソート
	sort.SliceStable(tl.events, func(i, j int) bool {
		return tl.events[i].Timestamp.Before(tl.events[j].Timestamp)
	})
	// 出力
	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		headerStyle.Render("TIME"),
		headerStyle.Render("Service Event"),
		headerStyle.Render("Message"),
	)
	fmt.Println(headerLine)

	for _, e := range tl.events {
		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			timestampStyle.Render(e.Timestamp.Format(time.RFC3339)),
			sourceStyle.Render(e.Source),
			messageStyle.Render(e.Message),
		)
		fmt.Println(line)
	}
}

// TimelineEvent を作る簡易ヘルパー
func newEvent(ts time.Time, source, msg string) TimelineEvent {
	return TimelineEvent{
		Timestamp: ts,
		Source:    source,
		Message:   msg,
	}
}
