package calendars

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	Gregorian = "gregorian"
	Shamsi    = "shamsi"
)

func Valid(name string) bool {
	return name == Gregorian || name == Shamsi
}

// Format converts ISO Gregorian dates used by resume entries to the selected
// display calendar. Free-form values are preserved unchanged.
func Format(value, calendar string) string {
	value = strings.TrimSpace(value)
	if value == "" || calendar != Shamsi {
		return value
	}
	formats := []struct {
		layout    string
		precision int
	}{
		{"2006-01-02", 3},
		{"2006/01/02", 3},
		{"2006-01", 2},
		{"2006/01", 2},
	}
	for _, format := range formats {
		date, err := time.Parse(format.layout, value)
		if err != nil {
			continue
		}
		year, month, day := gregorianToJalali(date.Year(), int(date.Month()), date.Day())
		if format.precision == 2 {
			return fmt.Sprintf("%04d/%02d", year, month)
		}
		return fmt.Sprintf("%04d/%02d/%02d", year, month, day)
	}
	// A year-only value has no exact cross-calendar equivalent. Resume entries
	// conventionally mean the year beginning, so display Farvardin of the
	// Jalali year that begins during that Gregorian year.
	if len(value) == 4 {
		if year, err := strconv.Atoi(value); err == nil && year >= 622 && year <= 9999 {
			return fmt.Sprintf("%04d/01", year-621)
		}
	}
	return value
}

func gregorianToJalali(year, month, day int) (int, int, int) {
	gDaysInMonth := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	jDaysInMonth := [...]int{31, 31, 31, 31, 31, 31, 30, 30, 30, 30, 30, 29}

	gy := year - 1600
	gm := month - 1
	gd := day - 1
	gDayNo := 365*gy + (gy+3)/4 - (gy+99)/100 + (gy+399)/400
	for i := 0; i < gm; i++ {
		gDayNo += gDaysInMonth[i]
	}
	if gm > 1 && ((gy%4 == 0 && gy%100 != 0) || gy%400 == 0) {
		gDayNo++
	}
	gDayNo += gd

	jDayNo := gDayNo - 79
	jNP := jDayNo / 12053
	jDayNo %= 12053
	jy := 979 + 33*jNP + 4*(jDayNo/1461)
	jDayNo %= 1461
	if jDayNo >= 366 {
		jy += (jDayNo - 1) / 365
		jDayNo = (jDayNo - 1) % 365
	}
	jm := 0
	for jm < 11 && jDayNo >= jDaysInMonth[jm] {
		jDayNo -= jDaysInMonth[jm]
		jm++
	}
	return jy, jm + 1, jDayNo + 1
}

func Normalize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if Valid(name) {
		return name
	}
	return Gregorian
}
