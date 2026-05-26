package messaging

import (
	"testing"
	"time"
)

// TestIsRead verifies BUG-C: a message sent in the same second it was read must
// count as "Read". The decision uses !t.After(othersLastRead) so an equal
// timestamp is Read, while a strictly-later timestamp is not.
func TestIsRead(t *testing.T) {
	read := time.Date(2026, 5, 22, 10, 30, 15, 0, time.UTC)
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		isOwn          bool
		sentAt         time.Time
		othersLastRead time.Time
		want           bool
	}{
		{
			name:           "own message before read is Read",
			isOwn:          true,
			sentAt:         read.Add(-time.Minute),
			othersLastRead: read,
			want:           true,
		},
		{
			name:           "own message at exact read time is Read",
			isOwn:          true,
			sentAt:         read,
			othersLastRead: read,
			want:           true,
		},
		{
			name:           "own message after read is not Read",
			isOwn:          true,
			sentAt:         read.Add(time.Second),
			othersLastRead: read,
			want:           false,
		},
		{
			name:           "not own message is never Read",
			isOwn:          false,
			sentAt:         read.Add(-time.Minute),
			othersLastRead: read,
			want:           false,
		},
		{
			name:           "epoch sentinel means never read",
			isOwn:          true,
			sentAt:         read.Add(-time.Hour),
			othersLastRead: epoch,
			want:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRead(tc.isOwn, tc.sentAt, tc.othersLastRead)
			if got != tc.want {
				t.Errorf("isRead(%v, %v, %v) = %v, want %v",
					tc.isOwn, tc.sentAt, tc.othersLastRead, got, tc.want)
			}
		})
	}
}
