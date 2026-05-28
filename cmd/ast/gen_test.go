package main

import (
	"reflect"
	"testing"
)

func TestExtractYAMLBlocks(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "single yaml block surrounded by prose",
			input: "Here are scenarios:\n\n```yaml\nid: one\nname: One\n```\n\nDone.",
			want: []string{"id: one\nname: One"},
		},
		{
			name:  "three blocks in a row, no prose",
			input: "```yaml\nid: a\n```\n```yaml\nid: b\n```\n```yaml\nid: c\n```\n",
			want:  []string{"id: a", "id: b", "id: c"},
		},
		{
			name:  "yml alias is accepted",
			input: "```yml\nid: x\n```",
			want:  []string{"id: x"},
		},
		{
			name:  "non-yaml fenced block is ignored",
			input: "```python\nprint('x')\n```\n```yaml\nid: only\n```",
			want:  []string{"id: only"},
		},
		{
			name:  "empty fenced block is dropped",
			input: "```yaml\n\n```\n```yaml\nid: real\n```",
			want:  []string{"id: real"},
		},
		{
			name:  "no fences returns empty slice",
			input: "Sorry, I can't generate scenarios.",
			want:  []string{},
		},
		{
			name:  "multi-line yaml is preserved verbatim",
			input: "```yaml\nid: deep\nassert:\n  output_text:\n    must_include:\n      - \"ok\"\n```",
			want:  []string{"id: deep\nassert:\n  output_text:\n    must_include:\n      - \"ok\""},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractYAMLBlocks(c.input)
			if len(c.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %#v, want %#v", got, c.want)
			}
		})
	}
}
