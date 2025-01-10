package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5f5f5")).Width(25).MarginRight(2)

	sourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00bfff")).Width(25)

	taskMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f5f5f5"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5f5f5")).Width(45).MarginBottom(2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00bfff")).
			Bold(true).Width(25).MarginRight(3)

	// ページング用のスタイル
	pagingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080"))
)

type Timeline struct {
	events   []TimelineEvent
	pageSize int
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

// イベントをソート
func (tl *Timeline) sortEvents() {
	sort.SliceStable(tl.events, func(i, j int) bool {
		return tl.events[i].Timestamp.After(tl.events[j].Timestamp)
	})
}

// ページのイベントを取得
func (tl *Timeline) getPageEvents(page int) []TimelineEvent {
	start := page * tl.pageSize
	end := min(start+tl.pageSize, len(tl.events))
	return tl.events[start:end]
}

// 1ページ分のイベントを描画
func (tl *Timeline) renderPage(events []TimelineEvent, currentPage, totalPages int) {

	// ヘッダー表示
	fmt.Println(lipgloss.JoinHorizontal(
		lipgloss.Top,
		headerStyle.Render("TIME"),
		headerStyle.Render("Log Source"),
		headerStyle.Render("Message"),
	))

	// イベント表示
	for _, e := range events {
		fmt.Println(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				timestampStyle.Render(e.Timestamp.Format("2006-01-02 15:04:05")),
				sourceStyle.Render(e.Source),
				messageStyle.Render(e.Message),
			))
	}

	// ページ情報表示
	pageText := fmt.Sprintf("Page %d/%d (Next ➡ Enter, Quit: q)",
		currentPage+1, totalPages)
	styledText := pagingStyle.Render(pageText)
	fmt.Println(styledText)
}

// メイン表示処理
func (tl *Timeline) Print() {
	tl.sortEvents()

	// ターミナルの行数を取得してページサイズを設定
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		log.Printf("failed to get window size: %v", err)
		return
	}
	tl.pageSize = height - 5 // ヘッダーとページ情報用に5行引く

	currentPage := 0
	totalPages := (len(tl.events) + tl.pageSize - 1) / tl.pageSize

	scanner := bufio.NewScanner(os.Stdin)
	for {
		// ページ描画
		events := tl.getPageEvents(currentPage)
		tl.renderPage(events, currentPage, totalPages)

		if !scanner.Scan() {
			// Ctrl+D or Ctrl+C
			return
		}
		input := scanner.Text()

		if input == "" {
			// Enterキーだけ押された場合 → 次のページへ
			if currentPage < totalPages-1 {
				currentPage++
			}
			continue
		} else if input == "q" {
			// q が入力された場合 → 終了
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TimelineEvent を作る簡易ヘルパー
func newEvent(ts time.Time, source, msg string) TimelineEvent {
	return TimelineEvent{
		Timestamp: ts,
		Source:    source,
		Message:   msg,
	}
}
