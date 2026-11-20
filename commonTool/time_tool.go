package commonTool

import (
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // 内置 IANA 时区数据，部署环境没有系统 zoneinfo 时仍可加载指定时区。
)

const (
	// DateTimeLayout 为本工具的秒级日期时间格式，不包含时区偏移或小数秒。
	DateTimeLayout = "2006-01-02 15:04:05"
	// DateLayout 为指定时区的日历日期格式。
	DateLayout = "2006-01-02"
)

// ErrAmbiguousLocalTime 表示夏令时回拨等原因使同一本地日期时间对应多个实际时刻。
// 调用方应使用带明确偏移的时间协议消除歧义，不自动选择其中一个时刻。
var ErrAmbiguousLocalTime = errors.New("local time is ambiguous")

// TimeTool 统一执行指定时区的日历边界、格式化和解析，可由多个请求并发复用。
// 未指定时区、零值或 nil 实例均使用 UTC（零时区），不读取或修改 time.Local。
type TimeTool struct {
	// location 在构造后只读；nil 默认 UTC，不随服务器本地时区变化。
	location *time.Location
}

// NewTimeTool 创建时区工具，timezone 为空默认 UTC；显式时区使用 IANA 名称，例如 Asia/Shanghai。
// 非法名称及依赖服务器配置的 Local 返回错误，不自动回退；调用方必须检查错误。
func NewTimeTool(timezone string) (*TimeTool, error) {
	if timezone == "" {
		return &TimeTool{location: time.UTC}, nil
	}
	if timezone == "Local" {
		return nil, errors.New("time tool requires an explicit timezone, Local is not allowed")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load time tool timezone: %w", err)
	}
	return &TimeTool{location: location}, nil
}

// DayStart 返回 base 在工具时区所属日的起点，base 的时刻不变，仅转换日历视角。
// 通常为本地 00:00；零点因时区跳变不存在时取该日首个有效时刻，重复零点取较早者。
func (t *TimeTool) DayStart(base time.Time) time.Time {
	local := base.In(t.loc())
	return t.calendarStart(local.Year(), local.Month(), local.Day())
}

// YesterdayStart 返回 base 所在时区的前一个日历日起点，不使用固定减 24 小时。
// 整个目标日期被时区变更跳过时，返回跳变后的首个有效时刻，该日对应空时间区间。
func (t *TimeTool) YesterdayStart(base time.Time) time.Time {
	local := base.In(t.loc())
	return t.calendarStart(local.Year(), local.Month(), local.Day()-1)
}

// MonthStart 返回 base 所在时区的本月起点，月份以工具时区的日历为准。
func (t *TimeTool) MonthStart(base time.Time) time.Time {
	local := base.In(t.loc())
	return t.calendarStart(local.Year(), local.Month(), 1)
}

// LastMonthStart 先定位月初再按日历回退一个月，避免 3 月 31 日减月被归一化回 3 月。
func (t *TimeTool) LastMonthStart(base time.Time) time.Time {
	local := base.In(t.loc())
	return t.calendarStart(local.Year(), local.Month()-1, 1)
}

// Date 返回 base 在工具时区的 YYYY-MM-DD 日期，不经过服务器本地时区转换。
func (t *TimeTool) Date(base time.Time) string {
	return base.In(t.loc()).Format(DateLayout)
}

// FormatUnix 将秒级 Unix 时间戳格式化为工具时区的 YYYY-MM-DD HH:MM:SS。
// 0 表示有效的 Unix 起始时刻，负数表示起点之前；不将 0 当作空值。
func (t *TimeTool) FormatUnix(seconds int64) string {
	return time.Unix(seconds, 0).In(t.loc()).Format(DateTimeLayout)
}

// Parse 将严格的 YYYY-MM-DD HH:MM:SS 按工具时区解析，保留底层 time.ParseError 错误链。
// 非法日期、非规范格式、时区跳变中不存在或重复的本地时刻均返回错误。
func (t *TimeTool) Parse(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(DateTimeLayout, value, t.loc())
	if err != nil {
		return time.Time{}, fmt.Errorf("parse local time: %w", err)
	}
	// 标准库允许布局未声明的小数秒，且可能归一化不存在的本地时刻；往返验证拒绝这些输入。
	if parsed.Format(DateTimeLayout) != value {
		return time.Time{}, errors.New("local time is not canonical or does not exist")
	}
	wall := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC)
	for _, point := range adjacentZoneTimes(parsed) {
		_, offset := point.Zone()
		candidate := wall.Add(-time.Duration(offset) * time.Second).In(t.loc())
		if !candidate.Equal(parsed) && candidate.Format(DateTimeLayout) == value {
			return time.Time{}, ErrAmbiguousLocalTime
		}
	}
	return parsed, nil
}

// DiffSeconds 返回 end-start 的实际秒数，允许负值；输入按同一工具时区解析，任何解析失败均返回错误。
// 直接计算秒级时间戳差，避免 time.Duration 在跨越约 292 年时发生饱和。
func (t *TimeTool) DiffSeconds(start, end string) (int64, error) {
	first, err := t.Parse(start)
	if err != nil {
		return 0, fmt.Errorf("parse start time: %w", err)
	}
	last, err := t.Parse(end)
	if err != nil {
		return 0, fmt.Errorf("parse end time: %w", err)
	}
	return last.Unix() - first.Unix(), nil
}

func (t *TimeTool) loc() *time.Location {
	if t == nil || t.location == nil {
		return time.UTC
	}
	return t.location
}

// calendarStart 使用 UTC 载体归一化年月日，再按目标时区寻找最早的有效日界。
// 检查相邻时区切换的偏移和边界，处理零点跳过、零点重复及整日跳过，避免 Date 单次归一化落到前一天。
func (t *TimeTool) calendarStart(year int, month time.Month, day int) time.Time {
	wall := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	location := t.loc()
	guess := time.Date(wall.Year(), wall.Month(), wall.Day(), 0, 0, 0, 0, location)
	best, found := guess, false
	consider := func(candidate time.Time) {
		localWall := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), candidate.Hour(), candidate.Minute(), candidate.Second(), 0, time.UTC)
		if !localWall.Before(wall) && (!found || candidate.Before(best)) {
			best, found = candidate, true
		}
	}
	for _, point := range adjacentZoneTimes(guess) {
		consider(point)
		_, offset := point.Zone()
		consider(wall.Add(-time.Duration(offset) * time.Second).In(location))
	}
	return best
}

// adjacentZoneTimes 收集当前区间及相邻切换两侧的时刻；无切换的固定时区仅返回自身。
func adjacentZoneTimes(base time.Time) []time.Time {
	points := []time.Time{base}
	start, end := base.ZoneBounds()
	for _, boundary := range []time.Time{start, end} {
		if !boundary.IsZero() {
			points = append(points, boundary.Add(-time.Second), boundary)
		}
	}
	return points
}
