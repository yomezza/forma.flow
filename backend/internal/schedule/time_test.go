package schedule

import (
	"testing"
	"time"
)

func TestDST(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	for _, date := range []string{"2026-03-29", "2026-10-25"} {
		if _, e := Instant(date, "02:30", loc); e == nil {
			t.Fatal("gap/fold accepted", date)
		}
	}
	if _, e := Instant("2026-03-28", "02:30", loc); e != nil {
		t.Fatal(e)
	}
	if _, e := Date("2026-02-30"); e == nil {
		t.Fatal("invalid date accepted")
	}
}
