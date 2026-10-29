package exportjob

import (
	"testing"
	"time"
)

func TestLoadWorkerConfig(t *testing.T) {
	tests := []struct {
		name               string
		workers, poll, jan string
		want               WorkerConfig
		wantErr            bool
	}{
		{name: "defaults", want: WorkerConfig{2, 2 * time.Second, 30 * time.Second}},
		{name: "custom", workers: "8", poll: "500ms", jan: "1m", want: WorkerConfig{8, 500 * time.Millisecond, time.Minute}},
		{name: "workers min", workers: "1", want: WorkerConfig{1, 2 * time.Second, 30 * time.Second}},
		{name: "workers zero", workers: "0", wantErr: true},
		{name: "workers too many", workers: "9", wantErr: true},
		{name: "workers nan", workers: "abc", wantErr: true},
		{name: "poll zero", poll: "0s", wantErr: true},
		{name: "poll negative", poll: "-1s", wantErr: true},
		{name: "poll garbage", poll: "2", wantErr: true},
		{name: "janitor over 1m", jan: "61s", wantErr: true},
		{name: "janitor zero", jan: "0", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EXPORT_WORKERS", tc.workers)
			t.Setenv("EXPORT_POLL_INTERVAL", tc.poll)
			t.Setenv("EXPORT_JANITOR_INTERVAL", tc.jan)
			got, err := LoadWorkerConfig()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
