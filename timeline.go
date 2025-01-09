package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(25)

	sourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Width(15).
			Bold(true)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).BorderBottom(true).Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(0, 1)
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
	fmt.Println(headerStyle.Render("TIMELINE (Service Events & Logs)"))
	fmt.Println()

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
