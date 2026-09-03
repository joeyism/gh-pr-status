package main

import "testing"

func TestPosixShellQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"hello", "'hello'"},
		{"https://github.com/org/repo/pull/1", "'https://github.com/org/repo/pull/1'"},
		{"foo'bar", "'foo'\\''bar'"},
		{"$HOME", "'$HOME'"},
		{"a`b", "'a`b'"},
	}
	for _, tc := range cases {
		got := posixShellQuote(tc.in)
		if got != tc.want {
			t.Errorf("posixShellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExpandOnEnter(t *testing.T) {
	url := "https://github.com/org/repo/pull/1"
	quoted := posixShellQuote(url)

	t.Run("substitutes placeholder", func(t *testing.T) {
		got := expandOnEnter("gh code-review ${PR_URL}", url)
		want := "gh code-review " + quoted
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
	t.Run("replaces every occurrence", func(t *testing.T) {
		got := expandOnEnter("echo ${PR_URL} ${PR_URL}", url)
		want := "echo " + quoted + " " + quoted
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
	t.Run("leaves unknown tokens alone", func(t *testing.T) {
		got := expandOnEnter("gh pr view ${PR_NUMBER}", url)
		if got != "gh pr view ${PR_NUMBER}" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("no placeholder returns template", func(t *testing.T) {
		got := expandOnEnter("true", url)
		if got != "true" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("does not expand unbraced $PR_URL", func(t *testing.T) {
		got := expandOnEnter("echo $PR_URL", url)
		if got != "echo $PR_URL" {
			t.Fatalf("got %q", got)
		}
	})
}
