package avatar

import "testing"

// Pure helpers in the handlers file — the HTTP plumbing is covered
// end-to-end by the smoke test; these tests pin the bits of logic
// that would otherwise be hidden behind a multipart request.

func TestExtensionFromFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"photo.png", "png"},
		{"portrait.JPG", "jpg"},
		{"image.jpeg", "jpeg"},
		{"archive.tar.gz", "gz"},
		{"noextension", ""},
		{"", ""},
		{".hidden", "hidden"},
	}
	for _, tc := range cases {
		if got := extensionFromFilename(tc.in); got != tc.want {
			t.Errorf("extensionFromFilename(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLastPathSegment(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/p/admin-avatars/user-1", "user-1"},
		{"/p/admin-avatars/user-1/", "user-1"},
		{"user-1", "user-1"},
		{"", ""},
		{"/", ""},
		{"/p/admin-avatars//user-1", "user-1"},
	}
	for _, tc := range cases {
		if got := lastPathSegment(tc.in); got != tc.want {
			t.Errorf("lastPathSegment(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestContentTypeForExt(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"png", "image/png"},
		{"jpg", "image/jpeg"},
		{"jpeg", "image/jpeg"},
		{"webp", "image/webp"},
		{"gif", "image/gif"},
		{"exe", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := contentTypeForExt(tc.in); got != tc.want {
			t.Errorf("contentTypeForExt(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}
