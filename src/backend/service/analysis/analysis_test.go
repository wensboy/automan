package analysis

import "testing"

func TestDomainOf(t *testing.T) {
	cases := map[string]string{
		"database: open":  "database",
		"proxy:start":     "proxy",
		"no separator":    "app",
		": leading colon": "app",
	}
	for input, want := range cases {
		if got := DomainOf(input); got != want {
			t.Fatalf("DomainOf(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRangeMillis(t *testing.T) {
	if rangeMillis("0.5h") != 30*60*1000 {
		t.Fatal("0.5h 应为 30 分钟")
	}
	if rangeMillis("unknown") != 60*60*1000 {
		t.Fatal("非法范围应回落 1h")
	}
}

func TestPlaceholders(t *testing.T) {
	if got := placeholders(2, 3); got != "(?,?,?),(?,?,?)" {
		t.Fatalf("placeholders = %q", got)
	}
}
