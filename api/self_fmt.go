package api

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// 人民币（分）转狗粮
func FormatRMB2Gouliang(input int64) int64 {
	return input
}

// e.g.  input=kevin  output=k***n
func FormatUserNameV1(userName string) string {
	s := []rune(userName)
	l := len(s)
	if l <= 1 {
		return "***" + userName
	}
	return fmt.Sprintf("%s***%s", string(s[0]), string(s[l-1]))
}

func FormatCommentTags(tags string) []string {
	return strings.Split(tags, ",")
}

func FormatAcceptOrderNumber(num int64) string {
	if num < 10000 {
		return fmt.Sprintf("%d", num)
	}
	return fmt.Sprintf("%.1f万", float64(num)/10000)
}

// 分转元, e.g. 100 > 1.00
func FormatPriceV1(points int64) string {
	return fmt.Sprintf("%.2f", float64(points)/100)
}

// 大神评分 e.g. 5 > 5.0
func FormatScore(score int64) string {
	return fmt.Sprintf("%.1f", float64(score))
}

// FormatAcceptOrderNumber2 FormatAcceptOrderNumber2
func FormatAcceptOrderNumber2(num int64) string {
	if num < 1000 {
		return fmt.Sprintf("%d", num)
	}

	v := fmt.Sprintf("%.4f", float64(num)/1000.0)
	return v[:len(v)-3] + "k"
}

// 日期转字符串
func FormatDatetime(t time.Time) string {
	return t.Format(time.RFC3339)
}
func FormatDatetimeV2(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// 浮点型精度处理
func Round(f float64, n int, roundDown bool) float64 {
	s := math.Pow10(n)
	if roundDown {
		return math.Floor(f*s) / s
	} else {
		return math.Ceil(f*s) / s
	}
}
