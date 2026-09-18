package rolemgmt

import "testing"

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "role", Message: "wajib diisi"}
	want := "role: wajib diisi"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
