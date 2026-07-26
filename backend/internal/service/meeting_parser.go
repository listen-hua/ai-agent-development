package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type meetingRequest struct {
	Intent        string
	Date          time.Time
	Times         []clockTime
	Duration      time.Duration
	Title         string
	AttendeeNames []string
	Capacity      int
}

type clockTime struct{ Hour, Minute int }

var (
	meetingDateYMD = regexp.MustCompile(`(?i)(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})(?:日)?`)
	meetingDateMD  = regexp.MustCompile(`(\d{1,2})月(\d{1,2})日?`)
	meetingTimes   = regexp.MustCompile(`(?:(上午|早上|中午|下午|晚上)\s*)?(\d{1,2})(?:[:：点时](\d{1,2})?分?)`)
	meetingMinutes = regexp.MustCompile(`(?:持续|时长)?\s*(\d+)\s*分钟`)
	meetingHours   = regexp.MustCompile(`(?:持续|时长)?\s*(\d+(?:\.\d+)?)\s*(?:个)?小时`)
	meetingPeople  = regexp.MustCompile(`(\d+)\s*人`)
	meetingRoomNo  = regexp.MustCompile(`(\d+)\s*号(?:会议室|洽谈室|会客室|室)?`)
)

func LooksLikeMeetingBooking(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	hasRoom := strings.Contains(text, "会议室") || strings.Contains(text, "会议")
	hasIntent := strings.Contains(text, "预约") || strings.Contains(text, "预订") || strings.Contains(text, "订一个") || strings.Contains(text, "取消") || strings.Contains(text, "改期") || strings.Contains(text, "改到") || strings.Contains(text, "调整")
	return hasRoom && hasIntent
}

func parseMeetingRequest(text string, now time.Time, location *time.Location) (meetingRequest, string) {
	request := meetingRequest{Intent: "create", Duration: 30 * time.Minute, Title: "会议"}
	if strings.Contains(text, "取消") {
		request.Intent = "cancel"
	} else if strings.Contains(text, "改期") || strings.Contains(text, "改到") || strings.Contains(text, "调整") {
		request.Intent = "reschedule"
	}
	request.Date = parseMeetingDate(text, now, location)
	if request.Date.IsZero() {
		return request, "请告诉我要预约或操作哪一天的会议室，例如“明天下午 3 点”。"
	}
	for _, match := range meetingTimes.FindAllStringSubmatch(text, -1) {
		hour, _ := strconv.Atoi(match[2])
		minute := 0
		if match[3] != "" {
			minute, _ = strconv.Atoi(match[3])
		}
		period := match[1]
		if (period == "下午" || period == "晚上") && hour < 12 {
			hour += 12
		}
		if period == "中午" && hour < 11 {
			hour += 12
		}
		if hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
			request.Times = append(request.Times, clockTime{Hour: hour, Minute: minute})
		}
	}
	if len(request.Times) >= 2 && looksLikeMeetingTimeRange(text) {
		start := request.Times[0]
		end := request.Times[1]
		startMinute := start.Hour*60 + start.Minute
		endMinute := end.Hour*60 + end.Minute
		if endMinute > startMinute {
			request.Duration = time.Duration(endMinute-startMinute) * time.Minute
			request.Times = request.Times[:1]
		}
	}
	if request.Intent != "cancel" && len(request.Times) == 0 {
		return request, "请告诉我明确的开始时间，例如“明天下午 3 点预约会议室”。"
	}
	if match := meetingMinutes.FindStringSubmatch(text); len(match) > 1 {
		minutes, _ := strconv.Atoi(match[1])
		request.Duration = time.Duration(minutes) * time.Minute
	} else if match := meetingHours.FindStringSubmatch(text); len(match) > 1 {
		hours, _ := strconv.ParseFloat(match[1], 64)
		request.Duration = time.Duration(hours * float64(time.Hour))
	}
	if request.Duration < 30*time.Minute {
		request.Duration = 30 * time.Minute
	}
	if match := meetingPeople.FindStringSubmatch(text); len(match) > 1 {
		request.Capacity, _ = strconv.Atoi(match[1])
	}
	request.AttendeeNames = parseAttendeeNames(text)
	if title := parseMeetingTitle(text); title != "" {
		request.Title = title
	}
	return request, ""
}

func looksLikeMeetingTimeRange(text string) bool {
	hasStart := strings.Contains(text, "从") || strings.Contains(text, "开始")
	hasEnd := strings.Contains(text, "到") || strings.Contains(text, "至") || strings.Contains(text, "结束")
	return hasStart && hasEnd
}

func parseMeetingDate(text string, now time.Time, location *time.Location) time.Time {
	now = now.In(location)
	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	switch {
	case strings.Contains(text, "后天"):
		return base.AddDate(0, 0, 2)
	case strings.Contains(text, "明天"):
		return base.AddDate(0, 0, 1)
	case strings.Contains(text, "今天"):
		return base
	}
	if match := meetingDateYMD.FindStringSubmatch(text); len(match) > 3 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
		if value.Year() == year && int(value.Month()) == month && value.Day() == day {
			return value
		}
	}
	if match := meetingDateMD.FindStringSubmatch(text); len(match) > 2 {
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		value := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, location)
		if value.Before(base) {
			value = value.AddDate(1, 0, 0)
		}
		if int(value.Month()) == month && value.Day() == day {
			return value
		}
	}
	return time.Time{}
}

func parseAttendeeNames(text string) []string {
	index := strings.Index(text, "参会人")
	if index < 0 {
		return nil
	}
	value := strings.TrimLeft(text[index+len("参会人"):], "：: ")
	for _, stop := range []string{"，预约", "，预订", "，会议室", "。", ";", "；"} {
		if i := strings.Index(value, stop); i >= 0 {
			value = value[:i]
		}
	}
	parts := regexp.MustCompile(`[、,，和\s]+`).Split(value, -1)
	values := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && len([]rune(part)) <= 20 {
			values = append(values, part)
		}
	}
	return values
}

func parseMeetingTitle(text string) string {
	for _, marker := range []string{"主题", "标题"} {
		if index := strings.Index(text, marker); index >= 0 {
			value := strings.TrimLeft(text[index+len(marker):], "：: ")
			if end := strings.IndexAny(value, "，。；;"); end >= 0 {
				value = value[:end]
			}
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}

func meetingDateTime(date time.Time, value clockTime) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), value.Hour, value.Minute, 0, 0, date.Location())
}

func meetingTimeLabel(start, end time.Time) string {
	return fmt.Sprintf("%s %s–%s", start.Format("01月02日"), start.Format("15:04"), end.Format("15:04"))
}
