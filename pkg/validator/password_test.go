package validator

import (
	"testing"
)

func TestGetRequirementsDescription(t *testing.T) {
	tests := []struct {
		name     string
		reqs     PasswordRequirements
		expected string
	}{
		{
			name: "all requirements enabled",
			reqs: PasswordRequirements{
				MinLength:      12,
				RequireUpper:   true,
				RequireLower:   true,
				RequireDigit:   true,
				RequireSpecial: true,
			},
			expected: "Must be at least 12 characters with uppercase, lowercase, digits, special characters",
		},
		{
			name: "only length required",
			reqs: PasswordRequirements{
				MinLength:      8,
				RequireUpper:   false,
				RequireLower:   false,
				RequireDigit:   false,
				RequireSpecial: false,
			},
			expected: "Must be at least 8 characters",
		},
		{
			name: "length and uppercase only",
			reqs: PasswordRequirements{
				MinLength:      10,
				RequireUpper:   true,
				RequireLower:   false,
				RequireDigit:   false,
				RequireSpecial: false,
			},
			expected: "Must be at least 10 characters with uppercase",
		},
		{
			name: "no special characters required",
			reqs: PasswordRequirements{
				MinLength:      12,
				RequireUpper:   true,
				RequireLower:   true,
				RequireDigit:   true,
				RequireSpecial: false,
			},
			expected: "Must be at least 12 characters with uppercase, lowercase, digits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reqs.GetRequirementsDescription()
			if result != tt.expected {
				t.Errorf("GetRequirementsDescription() = %q, want %q", result, tt.expected)
			}
		})
	}
}
