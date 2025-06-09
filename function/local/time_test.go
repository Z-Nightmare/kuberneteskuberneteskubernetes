package local

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestShowCurrentTimeZone(t *testing.T) {
	godotenv.Load("../../.env")
	// grep -q "^TZ=" .env && sed -i 's/^TZ=.*/TZ=Asia\/Shanghai/' .env || echo "TZ=Asia/Shanghai" >> .env
	fmt.Println(os.Getenv("TZ"))
	ShowCurrentTimeZone()
}
