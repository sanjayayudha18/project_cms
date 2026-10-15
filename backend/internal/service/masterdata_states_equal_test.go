package service

import "testing"

// statesEqual decides staleness at apply time (T2.5): a false "equal" applies
// an outdated edit, a false "different" blocks a valid one.
func TestStatesEqual(t *testing.T) {
	type state struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
	}
	current := state{ID: 7, Name: "ACM", IsActive: true}

	tests := []struct {
		name    string
		before  string
		current any
		want    bool
		wantErr bool
	}{
		{"same fields, same values", `{"id":7,"name":"ACM","is_active":true}`, current, true, false},
		{"field order does not matter", `{"is_active":true,"name":"ACM","id":7}`, current, true, false},
		{"value changed", `{"id":7,"name":"ACM OLD","is_active":true}`, current, false, false},
		{"flag changed", `{"id":7,"name":"ACM","is_active":false}`, current, false, false},
		{"field missing in snapshot", `{"id":7,"name":"ACM"}`, current, false, false},
		{"malformed snapshot", `{"id":`, current, false, true},
		{"unmarshalable current", `{}`, make(chan int), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := statesEqual([]byte(tt.before), tt.current)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("statesEqual = %v, want %v", got, tt.want)
			}
		})
	}
}
