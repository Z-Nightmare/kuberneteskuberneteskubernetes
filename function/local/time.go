package local

import (
	"fmt"
	"time"
)

// ShowCurrentTimeZone prints the current time zone and time in a friendly format.
func ShowCurrentTimeZone() {
	// godotenv.Load("../../.env")
	now := time.Now()
	zoneName, offset := now.Zone()
	offsetHours := offset / 3600
	offsetMinutes := (offset % 3600) / 60

	fmt.Println("🕒 当前时间信息:")
	fmt.Printf("📍 时区: %s (UTC%+02d:%02d)\n", zoneName, offsetHours, offsetMinutes)
	fmt.Printf("📅 当前时间: %s\n", now.Format("2006-01-02 15:04:05 MST"))
}
