// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

var stringBuilderPool = &sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// Format 文档 http://php.net/manual/en/function.date.php
// 目前没有支持的 S, L, o, B, v, e, I
func Format(format string, now ...time.Time) string {
	if len(now) == 0 {
		return Format(format, time.Now())
	}

	var t = now[0]

	// 快速识别的模板
	switch format {
	case "Y":
		return strconv.Itoa(t.Year())
	case "Y-m-d":
		return strconv.Itoa(t.Year()) + "-" + leadingZero(int(t.Month())) + "-" + leadingZero(t.Day())
	case "Y-m-d H:i:s":
		return strconv.Itoa(t.Year()) + "-" + leadingZero(int(t.Month())) + "-" + leadingZero(t.Day()) + " " + leadingZero(t.Hour()) + ":" + leadingZero(t.Minute()) + ":" + leadingZero(t.Second())
	case "Y/m/d H:i:s":
		return strconv.Itoa(t.Year()) + "/" + leadingZero(int(t.Month())) + "/" + leadingZero(t.Day()) + " " + leadingZero(t.Hour()) + ":" + leadingZero(t.Minute()) + ":" + leadingZero(t.Second())
	case "Ymd":
		return strconv.Itoa(t.Year()) + leadingZero(int(t.Month())) + leadingZero(t.Day())
	case "Ym":
		return strconv.Itoa(t.Year()) + leadingZero(int(t.Month()))
	case "Hi":
		return leadingZero(t.Hour()) + leadingZero(t.Minute())
	case "His":
		return leadingZero(t.Hour()) + leadingZero(t.Minute()) + leadingZero(t.Second())
	}

	// 自动构造
	var buffer = stringBuilderPool.Get().(*strings.Builder)
	if buffer.Len() > 0 {
		buffer.Reset()
	}
	defer func() {
		stringBuilderPool.Put(buffer)
	}()

	for _, runeItem := range format {
		switch runeItem {
		case 'Y': // 年份，比如：2016
			buffer.WriteString(strconv.Itoa(t.Year()))
		case 'y': // 年份后两位，比如：16
			buffer.WriteString(t.Format("06"))
		case 'm': // 01-12
			var m = int(t.Month())
			if m < 10 {
				buffer.WriteString("0" + strconv.Itoa(m))
			} else {
				buffer.WriteString(strconv.Itoa(m))
			}
		case 'n': // 1-12
			buffer.WriteString(strconv.Itoa(int(t.Month())))
		case 'd': // 01-31
			var d = t.Day()
			if d < 10 {
				buffer.WriteString("0" + strconv.Itoa(d))
			} else {
				buffer.WriteString(strconv.Itoa(d))
			}
		case 'z': // 0 -365
			buffer.WriteString(strconv.Itoa(t.YearDay() - 1))
		case 'j': // 1-31
			buffer.WriteString(strconv.Itoa(t.Day()))
		case 'H': // 00-23
			var h = t.Hour()
			if h == 0 {
				buffer.WriteString("00")
			} else if h < 10 {
				buffer.WriteString("0" + strconv.Itoa(h))
			} else {
				buffer.WriteString(strconv.Itoa(h))
			}
		case 'G': // 0-23
			buffer.WriteString(strconv.Itoa(t.Hour()))
		case 'g': // 小时：1-12
			buffer.WriteString(t.Format("3"))
		case 'h': // 小时：01-12
			buffer.WriteString(t.Format("03"))
		case 'i': // 00-59
			var m = t.Minute()
			if m == 0 {
				buffer.WriteString("00")
			} else if m < 10 {
				buffer.WriteString("0" + strconv.Itoa(m))
			} else {
				buffer.WriteString(strconv.Itoa(m))
			}
		case 's': // 00-59
			var s = t.Second()
			if s == 0 {
				buffer.WriteString("00")
			} else if s < 10 {
				buffer.WriteString("0" + strconv.Itoa(s))
			} else {
				buffer.WriteString(strconv.Itoa(s))
			}
		case 'A': // AM or PM
			buffer.WriteString(t.Format("PM"))
		case 'a': // am or pm
			buffer.WriteString(t.Format("pm"))
		case 'u': // 微秒：654321
			buffer.WriteString(strconv.Itoa(t.Nanosecond() / 1000))
		case 'v': // 毫秒：654
			buffer.WriteString(strconv.Itoa(t.Nanosecond() / 1000000))
		case 'w': // weekday, 0, 1, 2, ...
			buffer.WriteString(strconv.Itoa(int(t.Weekday())))
		case 'W': // ISO-8601 week，一年中第N周
			_, week := t.ISOWeek()
			buffer.WriteString(strconv.Itoa(week))
		case 'N': // 1, 2, ...7
			weekday := t.Weekday()
			if weekday == 0 {
				buffer.WriteString("7")
			} else {
				buffer.WriteString(strconv.Itoa(int(weekday)))
			}
		case 'D': // Mon ... Sun
			buffer.WriteString(t.Format("Mon"))
		case 'l': // Monday ... Sunday
			buffer.WriteString(t.Format("Monday"))
		case 't': // 一个月中的天数
			t2 := time.Date(t.Year(), t.Month(), 32, 0, 0, 0, 0, time.Local)
			daysInMonth := 32 - t2.Day()

			buffer.WriteString(strconv.Itoa(daysInMonth))
		case 'F': // January
			buffer.WriteString(t.Format("January"))
		case 'M': // Jan
			buffer.WriteString(t.Format("Jan"))
		case 'O': // 格林威治时间差（GMT），比如：+0800
			buffer.WriteString(t.Format("-0700"))
		case 'P': // 格林威治时间差（GMT），比如：+08:00
			buffer.WriteString(t.Format("-07:00"))
		case 'T': // 时区名，比如CST
			zone, _ := t.Zone()
			buffer.WriteString(zone)
		case 'Z': // 时区offset，比如28800
			_, offset := t.Zone()
			buffer.WriteString(strconv.Itoa(offset))
		case 'c': // ISO 8601，类似于：2004-02-12T15:19:21+00:00
			buffer.WriteString(t.Format("2006-01-02T15:04:05Z07:00"))
		case 'r': // RFC 2822，类似于：Thu, 21 Dec 2000 16:01:07 +0200
			buffer.WriteString(t.Format("Mon, 2 Jan 2006 15:04:05 -0700"))
		case 'U': // 时间戳
			buffer.WriteString(strconv.FormatInt(t.Unix(), 10))
		default:
			buffer.WriteRune(runeItem)
		}
	}

	return buffer.String()
}

// FormatTime 格式化时间戳
func FormatTime(format string, timestamp int64) string {
	return Format(format, time.Unix(timestamp, 0))
}

func leadingZero(i int) string {
	if i <= 0 {
		return "00"
	}
	var s = strconv.Itoa(i)
	if i < 10 {
		return "0" + s
	}
	return s
}
