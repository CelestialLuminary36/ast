package judge

import "testing"

func TestMatchGlob_Doublestar(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// 基础 *
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},

		// 前缀型 **（旧实现也能处理）
		{"src/**", "src/a.go", true},
		{"src/**", "src/a/b.go", true},
		{"src/**", "other/a.go", false},

		// 跨目录 **/*.ext —— 旧实现漏判的核心场景
		{"src/**/*.go", "src/a.go", true},
		{"src/**/*.go", "src/a/b.go", true},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "src/a.txt", false},
		{"src/**/*.go", "other/a.go", false},

		// 双 **
		{"**/test/**", "a/test/b.go", true},
		{"**/test/**", "test/a.go", true},
		{"**/test/**", "a/b/c.go", false},

		// 精确路径
		{"package-lock.json", "package-lock.json", true},
		{"package-lock.json", "sub/package-lock.json", false},
	}

	for _, c := range cases {
		got := matchGlob(c.pattern, c.path)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v; want %v", c.pattern, c.path, got, c.want)
		}
	}
}
