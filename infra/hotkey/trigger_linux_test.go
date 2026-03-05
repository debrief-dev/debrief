//go:build linux

package hotkey

import "testing"

type triggerTestCase struct {
	name    string
	modStrs []string
	keyStr  string
	want    string
}

type triggerBuilder struct {
	name  string
	fn    func([]string, string) string
	cases []triggerTestCase
}

func TestBuildTriggerStrings(t *testing.T) {
	builders := []triggerBuilder{
		{"GnomeTrigger", buildGnomeTrigger, []triggerTestCase{
			{"ctrl+shift+h", []string{Ctrl, Shift}, "H", "<Control><Shift>h"},
			{"alt+a", []string{Alt}, "A", "<Alt>a"},
			{"super+h", []string{Win}, "H", "<Super>h"},
			{"cmd+h", []string{Cmd}, "H", "<Super>h"},
			{"no modifiers", nil, "H", "h"},
		}},
		{"PortalTrigger", buildTriggerString, []triggerTestCase{
			{"ctrl+shift+h", []string{Ctrl, Shift}, "H", "CTRL+SHIFT+h"},
			{"alt+a", []string{Alt}, "A", "ALT+a"},
			{"super+h", []string{Win}, "H", "SUPER+h"},
			{"cmd+h", []string{Cmd}, "H", "SUPER+h"},
			{"no modifiers", nil, "H", "h"},
		}},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			for _, tt := range b.cases {
				t.Run(tt.name, func(t *testing.T) {
					got := b.fn(tt.modStrs, tt.keyStr)
					if got != tt.want {
						t.Errorf("got %q, want %q", got, tt.want)
					}
				})
			}
		})
	}
}
