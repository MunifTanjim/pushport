package transport

import "testing"

func TestPriorityMapping(t *testing.T) {
	cases := map[string]string{"high": "10", "normal": "10", "low": "5", "very-low": "5", "": "10"}
	for urg, want := range cases {
		if got := APNsPriority(urg); got != want {
			t.Fatalf("APNsPriority(%q)=%q want %q", urg, got, want)
		}
	}
	if FCMPriority("low") != "normal" || FCMPriority("high") != "high" || FCMPriority("") != "high" {
		t.Fatal("FCMPriority mapping wrong")
	}
	if NormalizeEncoding("") != "aes128gcm" || NormalizeEncoding("aesgcm") != "aesgcm" {
		t.Fatal("NormalizeEncoding wrong")
	}
}
