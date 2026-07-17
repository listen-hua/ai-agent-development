package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/model"
)

type reminderInterpretation struct {
	Intent        string
	Content       string
	Target        string
	Schedule      domain.ReminderSchedule
	Clarification string
}

var reminderTimePattern = regexp.MustCompile(`(凌晨|早上|上午|中午|下午|晚上)?\s*([0-9]{1,2}|[零〇一二两三四五六七八九十]{1,3})\s*[点时](?:\s*([0-9]{1,2}|[零〇一二两三四五六七八九十]{1,3})\s*分?)?`)

func (r *Reminder) interpret(ctx context.Context, input string, now time.Time) (reminderInterpretation, error) {
	if parsed, ok := deterministicReminder(input, now, r.location); ok {
		return parsed, nil
	}
	if r.provider == nil || !r.provider.Available() {
		return reminderInterpretation{}, fmt.Errorf("reminder parser model is unavailable")
	}
	system := fmt.Sprintf(`你是个人提醒命令解析器。当前时间是 %s，时区 Asia/Shanghai。只解析用户明确要求创建或管理自己的提醒的命令，以 JSON 输出，不要执行操作。
JSON 字段固定为：intent(create/list/update/pause/resume/delete/none)、content、target、schedule_type(once/daily/workday/weekly)、local_time(HH:MM)、weekdays(1到7，周一为1)、once_at(RFC3339)、clarification。
规则：没有明确提醒意图时 intent=none；上午下午不明确时填写 clarification；明确“今天”但时间已过时填写 clarification；不要猜测提醒对象。`, now.Format(time.RFC3339))
	answer, err := r.provider.Generate(ctx, model.GenerateRequest{Model: r.modelName, Temperature: 0, JSONMode: true, Messages: []model.Message{{Role: "system", Content: system}, {Role: "user", Content: input}}})
	if err != nil {
		return reminderInterpretation{}, err
	}
	var raw struct {
		Intent        string `json:"intent"`
		Content       string `json:"content"`
		Target        string `json:"target"`
		ScheduleType  string `json:"schedule_type"`
		LocalTime     string `json:"local_time"`
		Weekdays      []int  `json:"weekdays"`
		OnceAt        string `json:"once_at"`
		Clarification string `json:"clarification"`
	}
	if err = json.Unmarshal([]byte(answer), &raw); err != nil {
		return reminderInterpretation{}, err
	}
	parsed := reminderInterpretation{Intent: raw.Intent, Content: strings.TrimSpace(raw.Content), Target: strings.TrimSpace(raw.Target), Clarification: strings.TrimSpace(raw.Clarification)}
	if parsed.Intent == "none" || parsed.Intent == "list" || parsed.Intent == "pause" || parsed.Intent == "resume" || parsed.Intent == "delete" || parsed.Intent == "update" {
		return parsed, nil
	}
	if parsed.Intent != "create" {
		return reminderInterpretation{}, fmt.Errorf("unsupported reminder intent")
	}
	parsed.Schedule = domain.ReminderSchedule{Type: raw.ScheduleType, Timezone: "Asia/Shanghai", LocalTime: raw.LocalTime, Weekdays: raw.Weekdays}
	if raw.ScheduleType == domain.ReminderOnce {
		onceAt, parseErr := time.Parse(time.RFC3339, raw.OnceAt)
		if parseErr != nil {
			return reminderInterpretation{}, parseErr
		}
		parsed.Schedule.OnceAt = &onceAt
	}
	if err = parsed.Schedule.Validate(); err != nil {
		return reminderInterpretation{}, err
	}
	return parsed, nil
}

func deterministicReminder(input string, now time.Time, location *time.Location) (reminderInterpretation, bool) {
	trimmed := strings.TrimSpace(input)
	for _, intent := range []struct {
		Keyword string
		Value   string
	}{{"暂停提醒", "pause"}, {"恢复提醒", "resume"}, {"删除提醒", "delete"}, {"取消提醒", "delete"}} {
		if strings.Contains(trimmed, intent.Keyword) {
			target := strings.TrimSpace(strings.ReplaceAll(trimmed, intent.Keyword, ""))
			return reminderInterpretation{Intent: intent.Value, Target: target}, true
		}
	}
	cue := ""
	for _, value := range []string{"提醒我", "提醒一下", "别忘了", "叫我"} {
		if strings.Contains(trimmed, value) {
			cue = value
			break
		}
	}
	if cue == "" {
		return reminderInterpretation{}, false
	}
	match := reminderTimePattern.FindStringSubmatch(trimmed)
	if len(match) == 0 {
		return reminderInterpretation{Intent: "create", Clarification: "请补充具体提醒时间，例如“今天下午3点”。"}, true
	}
	hour, ok := parseChineseNumber(match[2])
	if !ok || hour > 23 {
		return reminderInterpretation{Intent: "create", Clarification: "提醒时间无法识别，请使用“上午9点”或“15:00”这样的表达。"}, true
	}
	minute := 0
	if match[3] != "" {
		minute, ok = parseChineseNumber(match[3])
		if !ok || minute > 59 {
			return reminderInterpretation{Intent: "create", Clarification: "提醒分钟数无法识别，请重新说明。"}, true
		}
	}
	period := match[1]
	if period == "下午" || period == "晚上" {
		if hour < 12 {
			hour += 12
		}
	} else if period == "中午" {
		if hour < 11 {
			hour += 12
		}
	} else if period == "" && hour >= 1 && hour <= 6 {
		return reminderInterpretation{Intent: "create", Clarification: fmt.Sprintf("你说的%d点是凌晨还是下午？请补充上午或下午。", hour)}, true
	}
	localTime := fmt.Sprintf("%02d:%02d", hour, minute)
	contentPart := trimmed
	if index := strings.Index(trimmed, cue); index >= 0 {
		contentPart = trimmed[index+len(cue):]
	}
	contentPart = reminderTimePattern.ReplaceAllString(contentPart, "")
	for _, token := range []string{"今天", "明天", "后天", "在工作日", "工作日", "每个", "每天", "每日", "每周一", "每周二", "每周三", "每周四", "每周五", "每周六", "每周日", "每周天", "每周"} {
		contentPart = strings.ReplaceAll(contentPart, token, "")
	}
	contentPart = strings.TrimSpace(strings.TrimLeft(contentPart, "，,。的要"))
	if contentPart == "" {
		return reminderInterpretation{Intent: "create", Clarification: "请补充要提醒的内容。"}, true
	}
	schedule := domain.ReminderSchedule{Timezone: "Asia/Shanghai", LocalTime: localTime}
	if strings.Contains(trimmed, "工作日") {
		schedule.Type = domain.ReminderWorkday
	} else if strings.Contains(trimmed, "每天") || strings.Contains(trimmed, "每日") {
		schedule.Type = domain.ReminderDaily
	} else if strings.Contains(trimmed, "每周") {
		schedule.Type = domain.ReminderWeekly
		weekday := parseWeekday(trimmed)
		if weekday == 0 {
			weekday = int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
		}
		schedule.Weekdays = []int{weekday}
	} else {
		schedule.Type = domain.ReminderOnce
		days := 0
		explicitToday := strings.Contains(trimmed, "今天")
		if strings.Contains(trimmed, "后天") {
			days = 2
		} else if strings.Contains(trimmed, "明天") {
			days = 1
		}
		date := now.AddDate(0, 0, days)
		candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location)
		if !candidate.After(now) {
			if explicitToday {
				return reminderInterpretation{Intent: "create", Clarification: "今天的这个时间已经过去，请重新指定时间。"}, true
			}
			candidate = candidate.AddDate(0, 0, 1)
		}
		schedule.OnceAt = &candidate
		schedule.LocalTime = ""
	}
	return reminderInterpretation{Intent: "create", Content: contentPart, Schedule: schedule}, true
}

func parseChineseNumber(value string) (int, bool) {
	if number, err := strconv.Atoi(value); err == nil {
		return number, true
	}
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	runes := []rune(value)
	if len(runes) == 1 {
		number, ok := digits[runes[0]]
		return number, ok
	}
	if len(runes) >= 2 && runes[0] == '十' {
		if len(runes) == 2 {
			number, ok := digits[runes[1]]
			return 10 + number, ok
		}
		return 10, true
	}
	if len(runes) >= 2 && runes[1] == '十' {
		tens, ok := digits[runes[0]]
		if !ok {
			return 0, false
		}
		number := tens * 10
		if len(runes) == 3 {
			ones, exists := digits[runes[2]]
			return number + ones, exists
		}
		return number, true
	}
	return 0, false
}

func parseWeekday(value string) int {
	for index, token := range []string{"一", "二", "三", "四", "五", "六", "日"} {
		if strings.Contains(value, "周"+token) || strings.Contains(value, "星期"+token) {
			return index + 1
		}
	}
	if strings.Contains(value, "周天") || strings.Contains(value, "星期天") {
		return 7
	}
	return 0
}
