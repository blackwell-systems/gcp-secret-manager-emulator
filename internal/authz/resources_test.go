package authz

import "testing"

func TestNormalizeSecretResource(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SecretPath",
			input: "projects/my-project/secrets/my-secret",
			want:  "projects/my-project/secrets/my-secret",
		},
		{
			name:  "VersionPath_StripVersion",
			input: "projects/my-project/secrets/my-secret/versions/1",
			want:  "projects/my-project/secrets/my-secret",
		},
		{
			name:  "VersionPath_StripLatest",
			input: "projects/my-project/secrets/my-secret/versions/latest",
			want:  "projects/my-project/secrets/my-secret",
		},
		{
			name:  "ShortPath_LessThan4Parts",
			input: "projects/my-project",
			want:  "projects/my-project",
		},
		{
			name:  "EmptyString",
			input: "",
			want:  "",
		},
		{
			name:  "SinglePart",
			input: "projects",
			want:  "projects",
		},
		{
			name:  "ThreeParts",
			input: "projects/my-project/secrets",
			want:  "projects/my-project/secrets",
		},
		{
			name:  "WrongPrefix_NotProjects",
			input: "organizations/my-org/secrets/my-secret",
			want:  "organizations/my-org/secrets/my-secret",
		},
		{
			name:  "WrongSecondTag_NotSecrets",
			input: "projects/my-project/topics/my-topic",
			want:  "projects/my-project/topics/my-topic",
		},
		{
			name:  "FourParts_CorrectFormat",
			input: "projects/p1/secrets/s1",
			want:  "projects/p1/secrets/s1",
		},
		{
			name:  "ExtraParts_AfterVersion",
			input: "projects/my-project/secrets/my-secret/versions/1/extra",
			want:  "projects/my-project/secrets/my-secret",
		},
		{
			name:  "FourParts_WrongStructure",
			input: "projects/my-project/buckets/my-bucket",
			want:  "projects/my-project/buckets/my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSecretResource(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeSecretResource(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSecretVersionResource(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "FullVersionPath",
			input: "projects/my-project/secrets/my-secret/versions/1",
			want:  "projects/my-project/secrets/my-secret/versions/1",
		},
		{
			name:  "LatestVersion",
			input: "projects/my-project/secrets/my-secret/versions/latest",
			want:  "projects/my-project/secrets/my-secret/versions/latest",
		},
		{
			name:  "SecretPath",
			input: "projects/my-project/secrets/my-secret",
			want:  "projects/my-project/secrets/my-secret",
		},
		{
			name:  "EmptyString",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSecretVersionResource(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeSecretVersionResource(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeParentForCreate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ProjectPath",
			input: "projects/my-project",
			want:  "projects/my-project",
		},
		{
			name:  "EmptyString",
			input: "",
			want:  "",
		},
		{
			name:  "LongerPath",
			input: "projects/my-project/locations/us-east1",
			want:  "projects/my-project/locations/us-east1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeParentForCreate(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeParentForCreate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
