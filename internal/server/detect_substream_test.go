package server

import "testing"

func TestSubstreamCandidates_SwapsSubtype(t *testing.T) {
	main := "rtsp://user:pass@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	got := substreamCandidates(main)
	want := "rtsp://user:pass@192.168.1.16:554/cam/realmonitor?channel=1&subtype=1"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("substreamCandidates = %v, want [%q]", got, want)
	}
}

func TestSubstreamCandidates_AppendsSubtypeWhenMissing(t *testing.T) {
	main := "rtsp://user:pass@192.168.1.16:554/cam/realmonitor?channel=1"
	got := substreamCandidates(main)
	want := "rtsp://user:pass@192.168.1.16:554/cam/realmonitor?channel=1&subtype=1"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("substreamCandidates = %v, want [%q]", got, want)
	}
}

func TestSubstreamCandidates_NoneWhenNoConvention(t *testing.T) {
	main := "rtsp://user:pass@10.0.0.9:554/Streaming/Channels/101"
	if got := substreamCandidates(main); len(got) != 0 {
		t.Fatalf("substreamCandidates = %v, want empty (no derivable convention)", got)
	}
}

func TestSubstreamCandidates_ExcludesMainURL(t *testing.T) {
	// A URL already at subtype=1 must not yield itself as a candidate.
	main := "rtsp://cam/realmonitor?channel=1&subtype=1"
	if got := substreamCandidates(main); len(got) != 0 {
		t.Fatalf("substreamCandidates = %v, want empty (candidate equals main)", got)
	}
}
